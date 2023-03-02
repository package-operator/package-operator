package handovers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/dynamiccache"
)

type reconciler interface {
	Reconcile(ctx context.Context, handover genericHandover) (ctrl.Result, error)
}

// Generic reconciler for both Handover and ClusterHandover objects.
type GenericHandoverController struct {
	newHandover  genericHandoverFactory
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

func NewClusterHandoverController(
	c client.Client, log logr.Logger,
	dynamicCache dynamicCache,
	scheme *runtime.Scheme,
) *GenericHandoverController {
	return newGenericHandoverController(
		newGenericClusterHandover,
		c, log, dynamicCache, scheme,
	)
}

func newGenericHandoverController(
	newHandover genericHandoverFactory,
	client client.Client, log logr.Logger,
	dynamicCache dynamicCache,
	scheme *runtime.Scheme,
) *GenericHandoverController {
	controller := &GenericHandoverController{
		newHandover:  newHandover,
		client:       client,
		dynamicCache: dynamicCache,
		log:          log,
		scheme:       scheme,
	}

	controller.reconciler = []reconciler{
		newAdoptionReconciler(client, dynamicCache),
		newRelabelReconciler(client, dynamicCache),
	}

	return controller
}

func (c *GenericHandoverController) SetupWithManager(mgr ctrl.Manager) error {
	handover := c.newHandover(c.scheme).ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(handover, builder.WithPredicates(&predicate.GenerationChangedPredicate{})).
		Watches(
			c.dynamicCache.Source(),
			&dynamiccache.EnqueueWatchingObjects{
				WatcherRefGetter: c.dynamicCache,
				WatcherType:      handover,
			},
		).
		Complete(c)
}

func (c *GenericHandoverController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("Handover", req.String())
	defer log.Info("reconciled")
	ctx = logr.NewContext(ctx, log)

	handover := c.newHandover(c.scheme)
	if err := c.client.Get(
		ctx, req.NamespacedName, handover.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	handoverClientObject := handover.ClientObject()
	if err := controllers.EnsureFinalizer(
		ctx, c.client, handoverClientObject, controllers.CachedFinalizer); err != nil {
		return ctrl.Result{}, err
	}

	if err := c.ensureWatch(ctx, handover); err != nil {
		return ctrl.Result{}, err
	}

	if !handoverClientObject.GetDeletionTimestamp().IsZero() {
		if err := c.handleDeletion(ctx, handover); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	var (
		res ctrl.Result
		err error
	)
	for _, r := range c.reconciler {
		res, err = r.Reconcile(ctx, handover)
		if err != nil || !res.IsZero() {
			break
		}
	}
	if err != nil {
		return res, err
	}
	return res, c.updateStatus(ctx, handover)
}

func (c *GenericHandoverController) updateStatus(ctx context.Context, handover genericHandover) error {
	handover.UpdatePhase()
	if err := c.client.Status().Update(ctx, handover.ClientObject()); err != nil {
		return fmt.Errorf("updating Handover status: %w", err)
	}
	return nil
}

// ensures the cache is watching the targetAPI.
func (c *GenericHandoverController) ensureWatch(
	ctx context.Context, handover genericHandover,
) error {
	gvk, objType, _ := controllers.UnstructuredFromTargetAPI(handover.GetTargetAPI())

	if err := c.dynamicCache.Watch(ctx, handover.ClientObject(), objType); err != nil {
		return fmt.Errorf("watching %s: %w", gvk, err)
	}
	return nil
}

func (c *GenericHandoverController) handleDeletion(
	ctx context.Context, handover genericHandover,
) error {
	if err := controllers.FreeCacheAndRemoveFinalizer(
		ctx, c.client, handover.ClientObject(), c.dynamicCache); err != nil {
		return err
	}

	return nil
}
