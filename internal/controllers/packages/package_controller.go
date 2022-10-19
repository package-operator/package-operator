package packages

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/ownerhandling"
)

type reconciler interface {
	Reconcile(ctx context.Context, pkg genericPackage) (ctrl.Result, error)
}

// Generic reconciler for both Package and ClusterPackage objects.
type GenericPackageController struct {
	newPackage genericPackageFactory

	client     client.Client
	log        logr.Logger
	scheme     *runtime.Scheme
	reconciler []reconciler

	jobOwnerStrategy ownerStrategy
}

type ownerStrategy interface {
	IsOwner(owner, obj metav1.Object) bool
	ReleaseController(obj metav1.Object)
	SetControllerReference(owner, obj metav1.Object) error
	EnqueueRequestForOwner(ownerType client.Object, isController bool) handler.EventHandler
}

func NewPackageController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme,
) *GenericPackageController {
	return newGenericPackageController(
		newGenericPackage,
		c, log, scheme, ownerhandling.NewAnnotation(scheme),
	)
}

func NewClusterPackageController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme,
) *GenericPackageController {
	return newGenericPackageController(
		newGenericClusterPackage,
		c, log, scheme, ownerhandling.NewNative(scheme),
	)
}

func newGenericPackageController(
	newPackage genericPackageFactory,
	client client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	jobOwnerStrategy ownerStrategy,
) *GenericPackageController {
	controller := &GenericPackageController{
		newPackage:       newPackage,
		client:           client,
		log:              log,
		scheme:           scheme,
		jobOwnerStrategy: jobOwnerStrategy,
	}

	controller.reconciler = []reconciler{
		&jobReconciler{
			scheme:           scheme,
			client:           client,
			newPackage:       newPackage,
			jobOwnerStrategy: jobOwnerStrategy,
		},
	}

	return controller
}

func (c *GenericPackageController) SetupWithManager(mgr ctrl.Manager) error {
	pkg := c.newPackage(c.scheme).ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(pkg).
		Watches(
			&source.Kind{
				Type: &batchv1.Job{},
			},
			c.jobOwnerStrategy.EnqueueRequestForOwner(pkg, true),
		).
		Watches(
			&source.Kind{Type: &corev1alpha1.ObjectDeployment{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &corev1alpha1.Package{},
			},
			builder.WithPredicates(predicate.Funcs{
				CreateFunc:  func(ce event.CreateEvent) bool { return false },
				GenericFunc: func(ge event.GenericEvent) bool { return false },
			}),
		).
		Complete(c)
}

func (c *GenericPackageController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("Package", req.String())
	defer log.Info("reconciled")
	ctx = logr.NewContext(ctx, log)

	pkg := c.newPackage(c.scheme)
	if err := c.client.Get(
		ctx, req.NamespacedName, pkg.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	pkgClientObject := pkg.ClientObject()

	if !pkgClientObject.GetDeletionTimestamp().IsZero() {
		for finalizer, cleaner := range FinalizersToCleaners {
			if err := cleaner(c.client, pkg); err != nil {
				return ctrl.Result{}, fmt.Errorf("error occurred while cleaning up the '%s' finalizer of PackageManifest: %w", finalizer, err)
			}
			if err := controllers.RemoveFinalizers(ctx, c.client, pkgClientObject, string(finalizer)); err != nil {
				return ctrl.Result{}, fmt.Errorf("error occurred while removing the '%s' finalizer from the PackageManifest: %w", finalizer, err)
			}
		}
		return ctrl.Result{}, nil
	}

	if err := controllers.EnsureFinalizers(ctx, c.client, pkgClientObject, packageFinalizers()...); err != nil {
		return ctrl.Result{}, err
	}
	var (
		res ctrl.Result
		err error
	)
	for _, r := range c.reconciler {
		res, err = r.Reconcile(ctx, pkg)
		if err != nil || !res.IsZero() {
			break
		}
	}
	if err != nil {
		return res, err
	}
	return res, c.updateStatus(ctx, pkg)
}

func (c *GenericPackageController) updateStatus(ctx context.Context, pkg genericPackage) error {
	pkg.UpdatePhase()
	if err := c.client.Status().Update(ctx, pkg.ClientObject()); err != nil {
		return fmt.Errorf("updating Package status: %w", err)
	}
	return nil
}
