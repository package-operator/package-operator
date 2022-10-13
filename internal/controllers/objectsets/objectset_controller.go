package objectsets

import (
	"context"
	"fmt"

	"package-operator.run/package-operator/internal/metrics"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
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

	recorder        metricsRecorder
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

type metricsRecorder interface {
	RecordRolloutTime(objectSet metrics.GenericObjectSet)
}

func NewObjectSetController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dw dynamicCache,
	r metricsRecorder,
) *GenericObjectSetController {
	return newGenericObjectSetController(
		newGenericObjectSet,
		newGenericObjectSetPhase,
		c, log, scheme, dw, r,
	)
}

func NewClusterObjectSetController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dw dynamicCache,
	r metricsRecorder,
) *GenericObjectSetController {
	return newGenericObjectSetController(
		newGenericClusterObjectSet,
		newGenericClusterObjectSetPhase,
		c, log, scheme, dw, r,
	)
}

func newGenericObjectSetController(
	newObjectSet genericObjectSetFactory,
	newObjectSetPhase genericObjectSetPhaseFactory,
	client client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicCache dynamicCache,
	recorder metricsRecorder,
) *GenericObjectSetController {
	controller := &GenericObjectSetController{
		newObjectSet:      newObjectSet,
		newObjectSetPhase: newObjectSetPhase,

		client:       client,
		log:          log,
		scheme:       scheme,
		dynamicCache: dynamicCache,
		recorder:     recorder,
	}

	phasesReconciler := newObjectSetPhasesReconciler(
		scheme,
		controllers.NewPhaseReconciler(
			scheme, client,
			dynamicCache, ownerhandling.NewNative(scheme)),
		newObjectSetRemotePhaseReconciler(
			client, scheme, newObjectSetPhase),
		controllers.NewPreviousRevisionLookup(
			scheme, func(s *runtime.Scheme) controllers.PreviousObjectSet {
				return newObjectSet(s)
			}, client).Lookup,
	)

	controller.teardownHandler = phasesReconciler

	controller.reconciler = []reconciler{
		&revisionReconciler{
			scheme:       scheme,
			client:       client,
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

	if err := c.reportPausedCondition(ctx, objectSet); err != nil {
		return res, fmt.Errorf("getting paused status: %w", err)
	}

	if c.recorder != nil {
		c.recorder.RecordRolloutTime(objectSet)
	}

	return res, c.updateStatus(ctx, objectSet)
}

func (c *GenericObjectSetController) updateStatus(ctx context.Context, objectSet genericObjectSet) error {
	objectSet.UpdateStatusPhase()
	if err := c.client.Status().Update(ctx, objectSet.ClientObject()); err != nil {
		return fmt.Errorf("updating ObjectSet status: %w", err)
	}
	return nil
}

func (c *GenericObjectSetController) reportPausedCondition(ctx context.Context, objectSet genericObjectSet) error {
	var phasesArePaused, unknown bool
	if len(objectSet.GetRemotePhases()) > 0 {
		var err error
		phasesArePaused, unknown, err = c.areRemotePhasesPaused(ctx, objectSet)
		if err != nil {
			return fmt.Errorf("getting status of remote phases: %w", err)
		}
	} else {
		phasesArePaused = objectSet.IsPaused()
	}

	switch {
	case unknown ||
		objectSet.IsPaused() && !phasesArePaused ||
		!objectSet.IsPaused() && phasesArePaused:
		// Could not get status of all remote ObjectSetPhases or they disagree with their parent.
		meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
			Type:    corev1alpha1.ObjectSetPaused,
			Status:  metav1.ConditionUnknown,
			Reason:  "PartiallyPaused",
			Message: "Waiting for ObjectSetPhases.",
		})

	case objectSet.IsPaused() && phasesArePaused:
		// Everything is paused!
		meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
			Type:    corev1alpha1.ObjectSetPaused,
			Status:  metav1.ConditionTrue,
			Reason:  "Paused",
			Message: "Lifecycle state set to paused.",
		})

	case !objectSet.IsPaused() && !phasesArePaused:
		// Nothing is paused!
		meta.RemoveStatusCondition(objectSet.GetConditions(), corev1alpha1.ObjectSetPaused)
	}
	return nil
}

func (c *GenericObjectSetController) areRemotePhasesPaused(ctx context.Context, objectSet genericObjectSet) (arePaused, unknown bool, err error) {
	var pausedPhases int
	for _, phaseRef := range objectSet.GetRemotePhases() {
		phase := c.newObjectSetPhase(c.scheme)
		err := c.client.Get(ctx, client.ObjectKey{
			Name:      phaseRef.Name,
			Namespace: objectSet.ClientObject().GetNamespace(),
		}, phase.ClientObject())
		if errors.IsNotFound(err) {
			// Phase object is not yet in cache, or was deleted by someone else.
			// -> we have to wait, but we don't want to raise an error in logs.
			return false, true, nil
		}
		if err != nil {
			return false, false, fmt.Errorf("get ObjectSetPhase: %w", err)
		}

		if meta.IsStatusConditionTrue(phase.GetConditions(), corev1alpha1.ObjectSetPhasePaused) {
			pausedPhases++
		}
	}
	arePaused = pausedPhases == len(objectSet.GetRemotePhases())
	return arePaused, false, nil
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
		objectSet.SetStatusControllerOf(nil) // we are no longer controlling anything.
	}

	return nil
}
