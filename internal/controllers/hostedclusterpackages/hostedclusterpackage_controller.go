package hostedclusterpackages

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
)

const (
	packageNameIndexKey = "metadata.name"
	minReadyDuration    = 30 * time.Second
)

type reconciler interface {
	Reconcile(ctx context.Context, hostedClusterPackage *corev1alpha1.HostedClusterPackage) (ctrl.Result, error)
}

type HostedClusterPackageController struct {
	client      client.Client
	log         logr.Logger
	scheme      *runtime.Scheme
	reconcilers []reconciler
}

func NewHostedClusterPackageController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
) *HostedClusterPackageController {
	return &HostedClusterPackageController{
		client: c,
		log:    log,
		scheme: scheme,
		reconcilers: []reconciler{
			&partitioningReconciler{
				client: c,
				log:    log,
				scheme: scheme,
				partReconciler: &partitionReconciler{
					client: c,
					log:    log,
					scheme: scheme,
				},
			},
		},
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

	for _, r := range c.reconcilers {
		res, err := r.Reconcile(ctx, hostedClusterPackage)
		if err != nil || !res.IsZero() {
			return res, err
		}
	}

	return c.updateStatus(ctx, hostedClusterPackage)
}

func (c *HostedClusterPackageController) updateStatus(
	ctx context.Context,
	hostedClusterPackage *corev1alpha1.HostedClusterPackage,
) (ctrl.Result, error) {
	hostedClusters := &v1beta1.HostedClusterList{}
	if err := c.client.List(ctx, hostedClusters, client.InNamespace("default")); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing clusters: %w", err)
	}

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
		len(hostedClusters.Items),
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
		Reason:             "TODO:Reason",
	})
	meta.SetStatusCondition(&hostedClusterPackage.Status.Conditions, metav1.Condition{
		Type:               corev1alpha1.HostedClusterPackageProgressing,
		Status:             progressing,
		ObservedGeneration: hostedClusterPackage.Generation,
		Reason:             "TODO:Reason",
	})
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
