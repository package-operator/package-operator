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

	return c.updateStatus(ctx, hostedClusterPackage, hostedClusters)
}

func (c *HostedClusterPackageController) updateStatus(
	ctx context.Context,
	hostedClusterPackage *corev1alpha1.HostedClusterPackage,
	hostedClusters *v1beta1.HostedClusterList,
) (ctrl.Result, error) {
	packages := &corev1alpha1.PackageList{}
	if err := c.client.List(ctx, packages, client.MatchingFields{
		packageNameIndexKey: hostedClusterPackage.Name,
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing packages: %w", err)
	}

	hostedClusterPackage.Status.ObservedGeneration = int32(hostedClusterPackage.GetGeneration())
	hostedClusterPackage.Status.Packages = int32(len(packages.Items))
	hostedClusterPackage.Status.AvailablePackages = 0
	hostedClusterPackage.Status.ReadyPackages = 0
	hostedClusterPackage.Status.UpdatedPackages = 0

	requeueAfter := 2 * minReadyDuration
	for _, pkg := range packages.Items {
		availableCond := meta.FindStatusCondition(pkg.Status.Conditions, corev1alpha1.PackageAvailable)
		if availableCond != nil && availableCond.Status == metav1.ConditionTrue {
			readyFor := time.Now().UTC().Sub(availableCond.LastTransitionTime.Time)
			if readyFor >= minReadyDuration {
				hostedClusterPackage.Status.AvailablePackages++
			} else {
				requeueAfter = min(requeueAfter, minReadyDuration-readyFor+time.Second)
			}
		}

		if meta.IsStatusConditionFalse(pkg.Status.Conditions, corev1alpha1.PackageProgressing) &&
			meta.IsStatusConditionTrue(pkg.Status.Conditions, corev1alpha1.PackageUnpacked) {
			hostedClusterPackage.Status.ReadyPackages++
		}

		if reflect.DeepEqual(pkg.Spec, hostedClusterPackage.Spec.PackageSpec) {
			hostedClusterPackage.Status.UpdatedPackages++
		}
	}

	hostedClusterPackage.Status.UnavailablePackages = int32(len(hostedClusters.Items)) -
		hostedClusterPackage.Status.AvailablePackages

	c.updateConditions(hostedClusterPackage)

	if err := c.client.Status().Update(ctx, hostedClusterPackage); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	if requeueAfter < 2*minReadyDuration {
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	return ctrl.Result{}, nil
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
		Reason:             "TODO:Reason",
	})
	meta.SetStatusCondition(&hostedClusterPackage.Status.Conditions, metav1.Condition{
		Type:               corev1alpha1.HostedClusterPackageProgressing,
		Status:             progressing,
		ObservedGeneration: hostedClusterPackage.Generation,
		Reason:             "TODO:Reason",
	})
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterPackage.Name,
			Namespace: v1beta1.HostedClusterNamespace(hc),
		},
		Spec: clusterPackage.Spec.PackageSpec,
	}

	if err := controllerutil.SetControllerReference(
		clusterPackage, pkg, c.scheme); err != nil {
		return nil, fmt.Errorf("setting controller reference: %w", err)
	}
	return pkg, nil
}

func (c *HostedClusterPackageController) SetupWithManager(mgr ctrl.Manager) error {
	// Index Packages by name
	if err := mgr.GetCache().IndexField(context.Background(), &corev1alpha1.Package{}, packageNameIndexKey,
		func(obj client.Object) []string {
			return []string{obj.GetName()}
		}); err != nil {
		return fmt.Errorf("creating name index for Packages: %w", err)
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
