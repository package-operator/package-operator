package packages

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"package-operator.run/package-operator/internal/adapters"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/packages/packagedeploy"
)

const loaderJobFinalizer = "package-operator.run/loader-job"

type reconciler interface {
	Reconcile(ctx context.Context, pkg adapters.GenericPackageAccessor) (ctrl.Result, error)
}

// Generic reconciler for both Package and ClusterPackage objects.
type GenericPackageController struct {
	newPackage          adapters.GenericPackageFactory
	newObjectDeployment adapters.ObjectDeploymentFactory

	client     client.Client
	log        logr.Logger
	scheme     *runtime.Scheme
	reconciler []reconciler
}

func NewPackageController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	imagePuller imagePuller,
) *GenericPackageController {
	return newGenericPackageController(
		adapters.NewGenericPackage, adapters.NewObjectDeployment,
		c, log, scheme, imagePuller, packagedeploy.NewPackageDeployer(c, scheme),
	)
}

func NewClusterPackageController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	imagePuller imagePuller,
) *GenericPackageController {
	return newGenericPackageController(
		adapters.NewGenericClusterPackage, adapters.NewClusterObjectDeployment,
		c, log, scheme, imagePuller, packagedeploy.NewClusterPackageDeployer(c, scheme),
	)
}

func newGenericPackageController(
	newPackage adapters.GenericPackageFactory,
	newObjectDeployment adapters.ObjectDeploymentFactory,
	client client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	imagePuller imagePuller,
	packageDeployer packageDeployer,
) *GenericPackageController {
	controller := &GenericPackageController{
		newPackage:          newPackage,
		newObjectDeployment: newObjectDeployment,
		client:              client,
		log:                 log,
		scheme:              scheme,
	}

	controller.reconciler = []reconciler{
		newUnpackReconciler(imagePuller, packageDeployer),
		&objectDeploymentStatusReconciler{
			client:              client,
			scheme:              scheme,
			newObjectDeployment: newObjectDeployment,
		},
	}

	return controller
}

func (c *GenericPackageController) SetupWithManager(mgr ctrl.Manager) error {
	pkg := c.newPackage(c.scheme).ClientObject()
	objDep := c.newObjectDeployment(c.scheme).ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		For(pkg).
		Owns(objDep).
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
		if err := c.handleDeletion(ctx, pkg); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
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

func (c *GenericPackageController) updateStatus(ctx context.Context, pkg adapters.GenericPackageAccessor) error {
	pkg.UpdatePhase()
	if err := c.client.Status().Update(ctx, pkg.ClientObject()); err != nil {
		return fmt.Errorf("updating Package status: %w", err)
	}
	return nil
}

func (c *GenericPackageController) handleDeletion(
	ctx context.Context, pkg adapters.GenericPackageAccessor,
) error {
	// Remove finalizer from previous versions of PKO.
	if err := controllers.RemoveFinalizer(
		ctx, c.client, pkg.ClientObject(), loaderJobFinalizer); err != nil {
		return err
	}

	return c.client.Update(ctx, pkg.ClientObject())
}
