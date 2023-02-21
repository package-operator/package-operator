package adoption

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/dynamiccache"
)

type reconciler interface {
	Reconcile(ctx context.Context, adoption genericAdoption) (ctrl.Result, error)
}

// Generic reconciler for both Adoption and ClusterAdoption objects.
type GenericAdoptionController struct {
	newAdoption genericAdoptionFactory

	client       client.Client
	log          logr.Logger
	scheme       *runtime.Scheme
	dynamicCache dynamicCache
	reconciler   []reconciler
}

type dynamicCache interface {
	client.Reader
	Source() source.Source
	Free(ctx context.Context, obj client.Object) error
	Watch(ctx context.Context, owner client.Object, obj runtime.Object) error
	OwnersForGKV(gvk schema.GroupVersionKind) []dynamiccache.OwnerReference
}

func NewAdoptionController(
	c client.Client, log logr.Logger,
	dynamicCache dynamicCache,
	scheme *runtime.Scheme,
) *GenericAdoptionController {
	return newGenericAdoptionController(
		newGenericAdoption,
		c, log, dynamicCache, scheme,
	)
}

func NewClusterAdoptionController(
	c client.Client, log logr.Logger,
	dynamicCache dynamicCache,
	scheme *runtime.Scheme,
) *GenericAdoptionController {
	return newGenericAdoptionController(
		newGenericClusterAdoption,
		c, log, dynamicCache, scheme,
	)
}

func newGenericAdoptionController(
	newAdoption genericAdoptionFactory,
	client client.Client, log logr.Logger,
	dynamicCache dynamicCache,
	scheme *runtime.Scheme,
) *GenericAdoptionController {
	controller := &GenericAdoptionController{
		newAdoption:  newAdoption,
		client:       client,
		dynamicCache: dynamicCache,
		log:          log,
		scheme:       scheme,
	}

	controller.reconciler = []reconciler{
		newStaticAdoptionReconciler(client, dynamicCache),
		newRoundRobinAdoptionReconciler(client, dynamicCache),
	}

	return controller
}

func (c *GenericAdoptionController) SetupWithManager(mgr ctrl.Manager) error {
	adoption := c.newAdoption(c.scheme).ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(adoption).
		Watches(
			c.dynamicCache.Source(),
			&dynamiccache.EnqueueWatchingObjects{
				WatcherRefGetter: c.dynamicCache,
				WatcherType:      adoption,
			},
		).
		Complete(c)
}

func (c *GenericAdoptionController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("Adoption", req.String())
	defer log.Info("reconciled")
	ctx = logr.NewContext(ctx, log)

	adoption := c.newAdoption(c.scheme)
	if err := c.client.Get(
		ctx, req.NamespacedName, adoption.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	adoptionClientObject := adoption.ClientObject()
	if err := controllers.EnsureFinalizer(
		ctx, c.client, adoptionClientObject, controllers.CachedFinalizer); err != nil {
		return ctrl.Result{}, err
	}

	if err := c.ensureWatch(ctx, adoption); err != nil {
		return ctrl.Result{}, err
	}

	if !adoptionClientObject.GetDeletionTimestamp().IsZero() {
		if err := c.handleDeletion(ctx, adoption); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	var (
		res ctrl.Result
		err error
	)
	for _, r := range c.reconciler {
		res, err = r.Reconcile(ctx, adoption)
		if err != nil || !res.IsZero() {
			break
		}
	}
	if err != nil {
		return res, err
	}
	return res, c.updateStatus(ctx, adoption)
}

func (c *GenericAdoptionController) updateStatus(ctx context.Context, adoption genericAdoption) error {
	adoption.UpdatePhase()
	if err := c.client.Status().Update(ctx, adoption.ClientObject()); err != nil {
		return fmt.Errorf("updating Adoption status: %w", err)
	}
	return nil
}

// ensures the cache is watching the targetAPI.
func (c *GenericAdoptionController) ensureWatch(
	ctx context.Context, adoption genericAdoption,
) error {
	gvk, objType, _ := controllers.UnstructuredFromTargetAPI(adoption.GetTargetAPI())

	if err := c.dynamicCache.Watch(ctx, adoption.ClientObject(), objType); err != nil {
		return fmt.Errorf("watching %s: %w", gvk, err)
	}
	return nil
}

func (c *GenericAdoptionController) handleDeletion(
	ctx context.Context, adoption genericAdoption,
) error {
	if err := controllers.FreeCacheAndRemoveFinalizer(
		ctx, c.client, adoption.ClientObject(), c.dynamicCache); err != nil {
		return err
	}

	return nil
}
