package packages

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/ownerhandling"
)

const loaderJobFinalizer = "package-operator.run/loader-job"

type reconciler interface {
	Reconcile(ctx context.Context, pkg genericPackage) (ctrl.Result, error)
}

// Generic reconciler for both Package and ClusterPackage objects.
type GenericPackageController struct {
	newPackage          genericPackageFactory
	newObjectDeployment genericObjectDeploymentFactory

	client     client.Client
	log        logr.Logger
	scheme     *runtime.Scheme
	reconciler []reconciler

	jobOwnerStrategy ownerStrategy
	pkoNamespace     string
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
	pkoNamespace, pkoImage string,
) *GenericPackageController {
	return newGenericPackageController(
		newGenericPackage, newGenericObjectDeployment,
		c, log, scheme, ownerhandling.NewAnnotation(scheme), pkoNamespace, pkoImage,
	)
}

func NewClusterPackageController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	pkoNamespace, pkoImage string,
) *GenericPackageController {
	return newGenericPackageController(
		newGenericClusterPackage, newGenericClusterObjectDeployment,
		c, log, scheme, ownerhandling.NewNative(scheme), pkoNamespace, pkoImage,
	)
}

func newGenericPackageController(
	newPackage genericPackageFactory,
	newObjectDeployment genericObjectDeploymentFactory,
	client client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	jobOwnerStrategy ownerStrategy,
	pkoNamespace, pkoImage string,
) *GenericPackageController {
	controller := &GenericPackageController{
		newPackage:          newPackage,
		newObjectDeployment: newObjectDeployment,
		client:              client,
		log:                 log,
		scheme:              scheme,
		jobOwnerStrategy:    jobOwnerStrategy,
		pkoNamespace:        pkoNamespace,
	}

	controller.reconciler = []reconciler{
		newJobReconciler(scheme, client, jobOwnerStrategy, pkoNamespace, pkoImage),
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
		For(pkg).
		Watches(
			&source.Kind{
				Type: &batchv1.Job{},
			},
			c.jobOwnerStrategy.EnqueueRequestForOwner(pkg, true),
		).
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
	if err := controllers.EnsureFinalizer(ctx, c.client, pkgClientObject, loaderJobFinalizer); err != nil {
		return ctrl.Result{}, err
	}

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

func (c *GenericPackageController) updateStatus(ctx context.Context, pkg genericPackage) error {
	pkg.UpdatePhase()
	if err := c.client.Status().Update(ctx, pkg.ClientObject()); err != nil {
		return fmt.Errorf("updating Package status: %w", err)
	}
	return nil
}

func (c *GenericPackageController) handleDeletion(
	ctx context.Context, pkg genericPackage,
) error {
	// Jobs need this setting or Pods created by a Job will not be deleted.
	background := metav1.DeletePropagationBackground
	if err := c.client.Delete(ctx, &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName(pkg), Namespace: c.pkoNamespace},
	}, &client.DeleteOptions{PropagationPolicy: &background}); err != nil && !errors.IsNotFound(err) {
		return err
	}

	if err := controllers.RemoveFinalizer(
		ctx, c.client, pkg.ClientObject(), loaderJobFinalizer); err != nil {
		return err
	}

	return c.client.Update(ctx, pkg.ClientObject())
}
