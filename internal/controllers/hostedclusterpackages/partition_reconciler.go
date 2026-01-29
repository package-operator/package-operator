package hostedclusterpackages

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
)

type partitionReconciler struct {
	client client.Client
	log    logr.Logger
	scheme *runtime.Scheme
}

func (r *partitionReconciler) Reconcile(
	ctx context.Context,
	hostedClusterPackage *corev1alpha1.HostedClusterPackage,
	part partition,
) (ctrl.Result, error) {
	for _, hc := range part.hostedClusters {
		if err := r.reconcileHostedCluster(ctx, hostedClusterPackage, hc); err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling hosted cluster '%s': %w", hc.Name, err)
		}
	}
	return r.updateStatus(ctx, hostedClusterPackage, part)
}

func (r *partitionReconciler) updateStatus(
	ctx context.Context,
	hostedClusterPackage *corev1alpha1.HostedClusterPackage,
	part partition,
) (ctrl.Result, error) {
	packages, err := r.listPackages(ctx, hostedClusterPackage, part)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("listing packages for partition '%s': %w", part.labelValue, err)
	}

	status := corev1alpha1.HostedClusterPackagePartitionStatus{
		Name: part.labelValue,
		HostedClusterPackageCountsStatus: corev1alpha1.HostedClusterPackageCountsStatus{
			ObservedGeneration: int32(hostedClusterPackage.GetGeneration()),
			Packages:           int32(len(packages)),
		},
	}

	res := updateStatusCounts(
		&status.HostedClusterPackageCountsStatus,
		hostedClusterPackage,
		packages,
		len(part.hostedClusters),
	)

	setPartitionStatus(hostedClusterPackage, part.labelValue, status)

	if !res.IsZero() {
		if err = r.client.Status().Update(ctx, hostedClusterPackage); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating status of partition '%s': %w", part.labelValue, err)
		}
	}

	return res, nil
}

func (r *partitionReconciler) reconcileHostedCluster(
	ctx context.Context,
	clusterPackage *corev1alpha1.HostedClusterPackage,
	hc v1beta1.HostedCluster,
) error {
	log := logr.FromContextOrDiscard(ctx)

	if !meta.IsStatusConditionTrue(hc.Status.Conditions, v1beta1.HostedClusterAvailable) {
		log.Info(fmt.Sprintf("waiting for HostedCluster '%s' to become ready", hc.Name))
		return nil
	}

	pkg, err := r.constructClusterPackage(clusterPackage, hc)
	if err != nil {
		return fmt.Errorf("constructing Package: %w", err)
	}

	existingPkg := &corev1alpha1.Package{}
	err = r.client.Get(ctx, client.ObjectKeyFromObject(pkg), existingPkg)
	if errors.IsNotFound(err) {
		if err := r.client.Create(ctx, pkg); err != nil {
			return fmt.Errorf("creating Package: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting Package: %w", err)
	}

	// Update package if spec is different.
	if !reflect.DeepEqual(existingPkg.Spec, pkg.Spec) {
		existingPkg.Spec = pkg.Spec
		if err := r.client.Update(ctx, existingPkg); err != nil {
			return fmt.Errorf("updating outdated Package: %w", err)
		}
	}

	return nil
}

func (r *partitionReconciler) constructClusterPackage(
	clusterPackage *corev1alpha1.HostedClusterPackage,
	hc v1beta1.HostedCluster,
) (*corev1alpha1.Package, error) {
	pkg := &corev1alpha1.Package{
		ObjectMeta: clusterPackage.Spec.Template.ObjectMeta,
		Spec:       clusterPackage.Spec.Template.Spec,
	}
	pkg.Name = clusterPackage.Name
	pkg.Namespace = v1beta1.HostedClusterNamespace(hc)

	if err := controllerutil.SetControllerReference(
		clusterPackage, pkg, r.scheme); err != nil {
		return nil, fmt.Errorf("setting controller reference: %w", err)
	}
	return pkg, nil
}

func (r *partitionReconciler) listPackages(
	ctx context.Context,
	hostedClusterPackage *corev1alpha1.HostedClusterPackage,
	part partition,
) ([]corev1alpha1.Package, error) {
	packages := &corev1alpha1.PackageList{}
	if err := r.client.List(ctx, packages, client.MatchingFields{
		packageNameIndexKey: hostedClusterPackage.Name,
	}); err != nil {
		return nil, fmt.Errorf("listing packages: %w", err)
	}

	return slices.DeleteFunc(packages.Items, func(pkg corev1alpha1.Package) bool {
		return !slices.ContainsFunc(part.hostedClusters, func(hc v1beta1.HostedCluster) bool {
			return v1beta1.HostedClusterNamespace(hc) == pkg.Namespace
		})
	}), nil
}

func setPartitionStatus(
	clusterPackage *corev1alpha1.HostedClusterPackage,
	name string, status corev1alpha1.HostedClusterPackagePartitionStatus,
) {
	i := slices.IndexFunc(clusterPackage.Status.Partitions,
		func(s corev1alpha1.HostedClusterPackagePartitionStatus) bool {
			return s.Name == name
		})
	if i == -1 {
		clusterPackage.Status.Partitions = append(clusterPackage.Status.Partitions, status)
	} else {
		clusterPackage.Status.Partitions[i] = status
	}
}
