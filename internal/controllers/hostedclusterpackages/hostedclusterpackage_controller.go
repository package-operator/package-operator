package hostedclusterpackages

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
)

const (
	packageNameIndexKey = "metadata.name"
	minReadyDuration    = 30 * time.Second
)

type HostedClusterPackageController struct {
	client client.Client
	log    logr.Logger
	scheme *runtime.Scheme
}

func NewHostedClusterPackageController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
) *HostedClusterPackageController {
	return &HostedClusterPackageController{
		client: c,
		log:    log,
		scheme: scheme,
	}
}

func (c *HostedClusterPackageController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("HostedClusterPackage", req.String())
	defer log.Info("reconciled")

	ctx = logr.NewContext(ctx, log)
	hostedClusterPackage := &corev1alpha1.HostedClusterPackage{}
	if err := c.client.Get(ctx, req.NamespacedName, hostedClusterPackage); err != nil {
		// Ignore not found errors on delete
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !hostedClusterPackage.DeletionTimestamp.IsZero() {
		log.Info("HostedClusterPackage is deleting")
		return ctrl.Result{}, nil
	}

	hostedClusters := &v1beta1.HostedClusterList{}
	if err := c.client.List(ctx, hostedClusters, client.InNamespace("default")); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing clusters: %w", err)
	}

	for _, hc := range hostedClusters.Items {
		if err := c.reconcileHostedCluster(ctx, hostedClusterPackage, hc); err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling hosted cluster '%s': %w", hc.Name, err)
		}
	}

	return c.updateStatus(ctx, hostedClusterPackage, len(hostedClusters.Items))
}

func (c *HostedClusterPackageController) reconcileHostedCluster(
	ctx context.Context,
	clusterPackage *corev1alpha1.HostedClusterPackage,
	hc v1beta1.HostedCluster,
) error {
	log := logr.FromContextOrDiscard(ctx)

	if !meta.IsStatusConditionTrue(hc.Status.Conditions, v1beta1.HostedClusterAvailable) {
		log.Info(fmt.Sprintf("waiting for HostedCluster '%s' to become ready", hc.Name))
		return nil
	}

	pkg, err := c.constructClusterPackage(clusterPackage, hc)
	if err != nil {
		return fmt.Errorf("constructing Package: %w", err)
	}

	existingPkg := &corev1alpha1.Package{}
	err = c.client.Get(ctx, client.ObjectKeyFromObject(pkg), existingPkg)
	if errors.IsNotFound(err) {
		if err := c.client.Create(ctx, pkg); err != nil {
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
		if err := c.client.Update(ctx, existingPkg); err != nil {
			return fmt.Errorf("updating outdated Package: %w", err)
		}
	}

	return nil
}

func (c *HostedClusterPackageController) constructClusterPackage(
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
		clusterPackage, pkg, c.scheme); err != nil {
		return nil, fmt.Errorf("setting controller reference: %w", err)
	}
	return pkg, nil
}

func (c *HostedClusterPackageController) updateStatus(
	ctx context.Context,
	hostedClusterPackage *corev1alpha1.HostedClusterPackage,
	hostedClusterCount int,
) (ctrl.Result, error) {
	packages := &corev1alpha1.PackageList{}
	if err := c.client.List(ctx, packages, client.MatchingFields{
		packageNameIndexKey: hostedClusterPackage.Name,
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing packages: %w", err)
	}

	res := updateStatusCounts(
		&hostedClusterPackage.Status.HostedClusterPackageCountsStatus,
		hostedClusterPackage,
		packages.Items,
		hostedClusterCount,
	)

	c.updateConditions(hostedClusterPackage)
	if err := c.client.Status().Update(ctx, hostedClusterPackage); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	return res, nil
}

func updateStatusCounts(
	counts *corev1alpha1.HostedClusterPackageCountsStatus,
	hostedClusterPackage *corev1alpha1.HostedClusterPackage,
	packages []corev1alpha1.Package,
	hostedClusters int,
) ctrl.Result {
	counts.ObservedGeneration = int32(hostedClusterPackage.GetGeneration())
	counts.Packages = int32(len(packages))
	counts.AvailablePackages = 0
	counts.ReadyPackages = 0
	counts.UpdatedPackages = 0

	requeueAfter := 2 * minReadyDuration
	for _, pkg := range packages {
		availableCond := meta.FindStatusCondition(pkg.Status.Conditions, corev1alpha1.PackageAvailable)
		if availableCond != nil && validateCondition(&pkg, corev1alpha1.PackageAvailable, metav1.ConditionTrue) {
			readyFor := time.Now().UTC().Sub(availableCond.LastTransitionTime.Time)
			if readyFor >= minReadyDuration {
				counts.AvailablePackages++
			} else {
				requeueAfter = min(requeueAfter, minReadyDuration-readyFor+time.Second)
			}
		}

		if validateCondition(&pkg, corev1alpha1.PackageProgressing, metav1.ConditionFalse) &&
			validateCondition(&pkg, corev1alpha1.PackageUnpacked, metav1.ConditionTrue) {
			counts.ReadyPackages++
		}

		if reflect.DeepEqual(pkg.Spec, hostedClusterPackage.Spec.Template.Spec) {
			counts.UpdatedPackages++
		}
	}

	counts.UnavailablePackages = int32(hostedClusters) - counts.AvailablePackages

	if requeueAfter < 2*minReadyDuration {
		return ctrl.Result{RequeueAfter: requeueAfter}
	}
	return ctrl.Result{}
}

func validateCondition(pkg *corev1alpha1.Package, conditionType string, status metav1.ConditionStatus) bool {
	cond := meta.FindStatusCondition(pkg.Status.Conditions, conditionType)
	return cond != nil && cond.Status == status && pkg.GetGeneration() == cond.ObservedGeneration
}

func (c *HostedClusterPackageController) updateConditions(hostedClusterPackage *corev1alpha1.HostedClusterPackage) {
	available := metav1.ConditionTrue
	progressing := metav1.ConditionTrue
	if hostedClusterPackage.Status.UnavailablePackages == 0 {
		progressing = metav1.ConditionFalse
	} else {
		available = metav1.ConditionFalse
	}

	meta.SetStatusCondition(&hostedClusterPackage.Status.Conditions, metav1.Condition{
		Type:               corev1alpha1.HostedClusterPackageAvailable,
		Status:             available,
		ObservedGeneration: hostedClusterPackage.Generation,
		Reason:             "Available",
	})
	meta.SetStatusCondition(&hostedClusterPackage.Status.Conditions, metav1.Condition{
		Type:               corev1alpha1.HostedClusterPackageProgressing,
		Status:             progressing,
		ObservedGeneration: hostedClusterPackage.Generation,
		Reason:             "Progressing",
	})
}

func (c *HostedClusterPackageController) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&corev1alpha1.Package{},
		packageNameIndexKey,
		func(obj client.Object) []string {
			return []string{obj.GetName()}
		},
	); err != nil {
		return fmt.Errorf("failed to setup field indexer: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.HostedClusterPackage{}).
		Owns(&corev1alpha1.Package{}).
		Watches(
			&v1beta1.HostedCluster{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []reconcile.Request {
				hcpkgList := &corev1alpha1.HostedClusterPackageList{}
				if err := c.client.List(ctx, hcpkgList); err != nil {
					return nil
				}

				// Enqueue all HostedClusterPackages on HostedCluster change
				requests := make([]reconcile.Request, len(hcpkgList.Items))
				for i, hcpkg := range hcpkgList.Items {
					requests[i] = reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name: hcpkg.Name,
						},
					}
				}

				return requests
			}),
		).
		Complete(c)
}
