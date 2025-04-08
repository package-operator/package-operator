package packages

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"package-operator.run/internal/adapters"
	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/environment"
	"package-operator.run/internal/imageprefix"
	"package-operator.run/internal/metrics"
	"package-operator.run/internal/packages"
)

const loaderJobFinalizer = "package-operator.run/loader-job"

var _ environment.Sinker = (*GenericPackageController)(nil)

type reconciler interface {
	Reconcile(ctx context.Context, pkg adapters.GenericPackageAccessor) (ctrl.Result, error)
}

type metricsRecorder interface {
	RecordPackageMetrics(pkg metrics.GenericPackage)
	RecordPackageLoadMetric(pkg metrics.GenericPackage, d time.Duration)
}

// Generic reconciler for both Package and ClusterPackage objects.
type GenericPackageController struct {
	newPackage          adapters.GenericPackageFactory
	newObjectDeployment adapters.ObjectDeploymentFactory

	recorder               metricsRecorder
	client                 client.Client
	log                    logr.Logger
	scheme                 *runtime.Scheme
	reconciler             []reconciler
	unpackReconciler       *unpackReconciler
	objDepStatusReconciler *objectDeploymentStatusReconciler
}

func NewPackageController(
	c client.Client, uncachedClient client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	imagePuller imagePuller,
	metricsRecorder metricsRecorder,
	packageHashModifier *int32,
	imagePrefixOverrides []imageprefix.Override,
) *GenericPackageController {
	return newGenericPackageController(
		adapters.NewGenericPackage, adapters.NewObjectDeployment,
		c, uncachedClient, log, scheme, imagePuller,
		packages.NewPackageDeployer(c, uncachedClient, scheme, imagePrefixOverrides),
		metricsRecorder, packageHashModifier, imagePrefixOverrides,
	)
}

func NewClusterPackageController(
	c client.Client, uncachedClient client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	imagePuller imagePuller,
	metricsRecorder metricsRecorder,
	packageHashModifier *int32,
	imagePrefixOverrides []imageprefix.Override,
) *GenericPackageController {
	return newGenericPackageController(
		adapters.NewGenericClusterPackage, adapters.NewClusterObjectDeployment,
		c, uncachedClient, log, scheme, imagePuller, packages.NewClusterPackageDeployer(c, scheme, imagePrefixOverrides),
		metricsRecorder, packageHashModifier, imagePrefixOverrides,
	)
}

func newGenericPackageController(
	newPackage adapters.GenericPackageFactory,
	newObjectDeployment adapters.ObjectDeploymentFactory,
	client client.Client, uncachedClient client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	imagePuller imagePuller,
	packageDeployer packageDeployer,
	metricsRecorder metricsRecorder,
	packageHashModifier *int32,
	imagePrefixOverrides []imageprefix.Override,
) *GenericPackageController {
	controller := &GenericPackageController{
		newPackage:          newPackage,
		newObjectDeployment: newObjectDeployment,
		recorder:            metricsRecorder,
		client:              client,
		log:                 log,
		scheme:              scheme,
		unpackReconciler: newUnpackReconciler(
			uncachedClient, imagePuller, packageDeployer,
			metricsRecorder, environment.NewSink(client), packageHashModifier,
			imagePrefixOverrides,
		),
		objDepStatusReconciler: &objectDeploymentStatusReconciler{
			client:              client,
			scheme:              scheme,
			newObjectDeployment: newObjectDeployment,
		},
	}

	controller.reconciler = []reconciler{
		controller.unpackReconciler,
		controller.objDepStatusReconciler,
	}

	return controller
}

func (c *GenericPackageController) SetEnvironment(env *manifests.PackageEnvironment) {
	c.unpackReconciler.SetEnvironment(env)
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
) (res ctrl.Result, err error) {
	log := c.log.WithValues("Package", req.String())
	defer log.Info("reconciled")
	ctx = logr.NewContext(ctx, log)

	pkg := c.newPackage(c.scheme)
	if err := c.client.Get(
		ctx, req.NamespacedName, pkg.ClientObject()); err != nil {
		return res, client.IgnoreNotFound(err)
	}
	defer func() {
		if err != nil {
			return
		}
		if c.recorder != nil {
			c.recorder.RecordPackageMetrics(pkg)
		}
	}()

	pkgClientObject := pkg.ClientObject()
	if !pkgClientObject.GetDeletionTimestamp().IsZero() {
		if err := c.handleDeletion(ctx, pkg); err != nil {
			return res, err
		}
		return res, nil
	}

	objDep := c.newObjectDeployment(c.scheme)
	err = c.client.Get(ctx, client.ObjectKeyFromObject(pkgClientObject), objDep.ClientObject())
	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}

	if pkg.GetSpecPaused() != objDep.GetSpecPaused() {
		var pauseChangeMsg string
		if pkg.GetSpecPaused() {
			objDep.SetSpecPaused(true)
			pauseChangeMsg = "pause"
		} else {
			objDep.SetSpecPaused(false)
			pauseChangeMsg = "unpause"
		}
		if err = c.client.Update(ctx, objDep.ClientObject()); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to %s objectdeployment: %w", pauseChangeMsg, err)
		}
	}

	// Skip subreconcilers when paused
	if pkg.GetSpecPaused() {
		res, err = c.objDepStatusReconciler.Reconcile(ctx, pkg)
		if err != nil {
			return res, err
		}
		return res, c.updateStatus(ctx, pkg)
	}

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
