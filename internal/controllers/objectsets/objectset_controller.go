package objectsets

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/ownerhandling"
)

// Generic reconciler for both ObjectSet and ClusterObjectSet objects.
type GenericObjectSetController struct {
	newObjectSet      genericObjectSetFactory
	newObjectSetPhase genericObjectSetPhaseFactory

	client     client.Client
	log        logr.Logger
	scheme     *runtime.Scheme
	reconciler []reconciler

	dynamicCache    dynamicCache
	teardownHandler teardownHandler
}

type reconciler interface {
	Reconcile(ctx context.Context, objectSet genericObjectSet) (ctrl.Result, error)
}

type dynamicCache interface {
	client.Reader
	Source() source.Source
	Free(ctx context.Context, obj client.Object) error
	Watch(ctx context.Context, owner client.Object, obj runtime.Object) error
}

type teardownHandler interface {
	Teardown(
		ctx context.Context, objectSet genericObjectSet,
	) (cleanupDone bool, err error)
}

func NewObjectSetController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dw dynamicCache,
) *GenericObjectSetController {
	return newGenericObjectSetController(
		newGenericObjectSet,
		newGenericObjectSetPhase,
		c, log, scheme, dw,
	)
}

func NewClusterObjectSetController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dw dynamicCache,
) *GenericObjectSetController {
	return newGenericObjectSetController(
		newGenericClusterObjectSet,
		newGenericClusterObjectSetPhase,
		c, log, scheme, dw,
	)
}

func newGenericObjectSetController(
	newObjectSet genericObjectSetFactory,
	newObjectSetPhase genericObjectSetPhaseFactory,
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicCache dynamicCache,
) *GenericObjectSetController {
	controller := &GenericObjectSetController{
		newObjectSet:      newObjectSet,
		newObjectSetPhase: newObjectSetPhase,

		client:       c,
		log:          log,
		scheme:       scheme,
		dynamicCache: dynamicCache,
	}

	phasesReconciler := newPhasesReconciler(c, controllers.NewPhaseReconciler(
		scheme, c, dynamicCache, ownerhandling.NewNative(scheme),
	), scheme, newObjectSet)

	controller.teardownHandler = phasesReconciler

	controller.reconciler = []reconciler{
		&revisionReconciler{
			scheme:       scheme,
			client:       c,
			newObjectSet: newObjectSet,
		},
		phasesReconciler,
	}

	return controller
}

func (c *GenericObjectSetController) SetupWithManager(mgr ctrl.Manager) error {
	objectSet := c.newObjectSet(c.scheme).ClientObject()
	objectSetPhase := c.newObjectSetPhase(c.scheme).ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(objectSet).
		Owns(objectSetPhase).
		Watches(c.dynamicCache.Source(), &handler.EnqueueRequestForOwner{
			OwnerType:    objectSet,
			IsController: false,
		}).
		Complete(c)
}

func (c *GenericObjectSetController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("ObjectSet", req.String())
	defer log.Info("reconciled")
	ctx = logr.NewContext(ctx, log)

	objectSet := c.newObjectSet(c.scheme)
	if err := c.client.Get(
		ctx, req.NamespacedName, objectSet.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if meta.IsStatusConditionTrue(*objectSet.GetConditions(), corev1alpha1.ObjectSetArchived) {
		// We don't want to touch this object anymore.
		return ctrl.Result{}, nil
	}

	if !objectSet.ClientObject().GetDeletionTimestamp().IsZero() ||
		objectSet.IsArchived() {
		if err := c.handleDeletionAndArchival(ctx, objectSet); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, c.updateStatus(ctx, objectSet)
	}

	if err := controllers.EnsureCachedFinalizer(ctx, c.client, objectSet.ClientObject()); err != nil {
		return ctrl.Result{}, err
	}

	var (
		res ctrl.Result
		err error
	)
	for _, r := range c.reconciler {
		res, err = r.Reconcile(ctx, objectSet)
		if err != nil || !res.IsZero() {
			break
		}
	}
	if err != nil {
		return res, err
	}

	c.reportPausedCondition(ctx, objectSet)
	return res, c.updateStatus(ctx, objectSet)
}

func (c *GenericObjectSetController) updateStatus(ctx context.Context, objectSet genericObjectSet) error {
	objectSet.UpdateStatusPhase()
	// this controller owns status alone, so we can always update it without optimistic locking.
	objectSet.ClientObject().SetResourceVersion("")
	if err := c.client.Status().Patch(ctx, objectSet.ClientObject(), client.Merge); err != nil {
		return fmt.Errorf("updating ObjectSet status: %w", err)
	}
	return nil
}

func (c *GenericObjectSetController) reportPausedCondition(ctx context.Context, objectSet genericObjectSet) {
	if objectSet.IsPaused() {
		meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
			Type:    corev1alpha1.ObjectSetPaused,
			Status:  metav1.ConditionTrue,
			Reason:  "Paused",
			Message: "Lifecycle state set to paused.",
		})
	} else {
		meta.RemoveStatusCondition(objectSet.GetConditions(), corev1alpha1.ObjectSetPaused)
	}
}

func (c *GenericObjectSetController) handleDeletionAndArchival(
	ctx context.Context, objectSet genericObjectSet,
) error {
	// always make sure to remove Available condition
	defer meta.RemoveStatusCondition(objectSet.GetConditions(), corev1alpha1.ObjectSetAvailable)

	done, err := c.teardownHandler.Teardown(ctx, objectSet)
	if err != nil {
		return fmt.Errorf("error tearing down during deletion: %w", err)
	}

	if !done {
		if objectSet.IsArchived() {
			meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
				Type:               corev1alpha1.ObjectSetArchived,
				Status:             metav1.ConditionFalse,
				Reason:             "ArchivalInProgress",
				Message:            "Object teardown in progress.",
				ObservedGeneration: objectSet.ClientObject().GetGeneration(),
			})
		}
		// don't remove finalizer before deletion is done
		return nil
	}

	if err := controllers.FreeCacheAndRemoveFinalizer(
		ctx, c.client, objectSet.ClientObject(), c.dynamicCache); err != nil {
		return err
	}

	// Needs to be called _after_ FreeCacheAndFinalizer,
	// because .Update is loading new state into objectSet, overriding changes to conditions.
	if objectSet.IsArchived() {
		meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetArchived,
			Status:             metav1.ConditionTrue,
			Reason:             "Archived",
			ObservedGeneration: objectSet.ClientObject().GetGeneration(),
		})
	}

	return nil
}
