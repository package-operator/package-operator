package objectsets

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/adapters"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/metrics"
	"package-operator.run/package-operator/internal/ownerhandling"
	"package-operator.run/package-operator/internal/preflight"
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
	RecordObjectSetMetrics(objectSet metrics.GenericObjectSet)
}

func NewObjectSetController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	dw dynamicCache, uc client.Reader,
	r metricsRecorder, restMapper meta.RESTMapper,
) *GenericObjectSetController {
	return newGenericObjectSetController(
		newGenericObjectSet,
		newGenericObjectSetPhase,
		adapters.NewObjectSlice,
		c, log, scheme, dw, uc, r,
		restMapper,
	)
}

func NewClusterObjectSetController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	dw dynamicCache, uc client.Reader,
	r metricsRecorder, restMapper meta.RESTMapper,
) *GenericObjectSetController {
	return newGenericObjectSetController(
		newGenericClusterObjectSet,
		newGenericClusterObjectSetPhase,
		adapters.NewClusterObjectSlice,
		c, log, scheme, dw, uc, r,
		restMapper,
	)
}

func newGenericObjectSetController(
	newObjectSet genericObjectSetFactory,
	newObjectSetPhase genericObjectSetPhaseFactory,
	newObjectSlice adapters.ObjectSliceFactory,
	client client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	dynamicCache dynamicCache, uncachedClient client.Reader,
	recorder metricsRecorder, restMapper meta.RESTMapper,
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
			dynamicCache,
			uncachedClient,
			ownerhandling.NewNative(scheme),
			preflight.List{
				preflight.NewAPIExistence(restMapper),
				preflight.NewNamespaceEscalation(restMapper),
				preflight.NewDryRun(client),
			},
		),
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
		newObjectSliceLoadReconciler(scheme, client, newObjectSlice),
		phasesReconciler,
	}

	return controller
}

func (c *GenericObjectSetController) SetupWithManager(mgr ctrl.Manager) error {
	objectSet := c.newObjectSet(c.scheme).ClientObject()
	objectSetPhase := c.newObjectSetPhase(c.scheme).ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(objectSet, builder.WithPredicates(&predicate.GenerationChangedPredicate{})).
		Owns(objectSetPhase).
		WatchesRawSource(
			c.dynamicCache.Source(),
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), objectSet),
			builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
				c.log.Info(
					"processing dynamic cache event",
					"object", client.ObjectKeyFromObject(object),
					"owners", object.GetOwnerReferences())
				return true
			}))).
		Complete(c)
}

func (c *GenericObjectSetController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (res ctrl.Result, err error) {
	log := c.log.WithValues("ObjectSet", req.String())
	defer log.Info("reconciled")
	ctx = logr.NewContext(ctx, log)

	objectSet := c.newObjectSet(c.scheme)
	if err := c.client.Get(
		ctx, req.NamespacedName, objectSet.ClientObject()); err != nil {
		return res, client.IgnoreNotFound(err)
	}
	defer func() {
		if err != nil {
			return
		}
		if c.recorder != nil {
			c.recorder.RecordObjectSetMetrics(objectSet)
		}
	}()

	if meta.IsStatusConditionTrue(*objectSet.GetConditions(), corev1alpha1.ObjectSetArchived) {
		// We don't want to touch this object anymore.
		return res, nil
	}

	if !objectSet.ClientObject().GetDeletionTimestamp().IsZero() ||
		objectSet.IsArchived() {
		if err := c.handleDeletionAndArchival(ctx, objectSet); err != nil {
			return res, err
		}

		if !objectSet.IsArchived() {
			// Object was deleted an not just archived.
			// no way to update status now :)
			return res, nil
		}

		return res, c.updateStatus(ctx, objectSet)
	}

	if err := controllers.EnsureCachedFinalizer(ctx, c.client, objectSet.ClientObject()); err != nil {
		return res, err
	}

	for _, r := range c.reconciler {
		res, err = r.Reconcile(ctx, objectSet)
		if err != nil || !res.IsZero() {
			break
		}
	}
	if err != nil {
		return res, c.updateStatusError(ctx, objectSet, err)
	}

	if err := c.reportPausedCondition(ctx, objectSet); err != nil {
		return res, fmt.Errorf("getting paused status: %w", err)
	}

	return res, c.updateStatus(ctx, objectSet)
}

func (c *GenericObjectSetController) updateStatusError(ctx context.Context, objectSet genericObjectSet,
	reconcileErr error,
) error {
	var preflightError *preflight.Error
	if errors.As(reconcileErr, &preflightError) {
		meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetAvailable,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: objectSet.GetGeneration(),
			Reason:             "PreflightError",
			Message:            preflightError.Error(),
		})
		return c.updateStatus(ctx, objectSet)
	}
	return reconcileErr
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
			Type:               corev1alpha1.ObjectSetPaused,
			Status:             metav1.ConditionUnknown,
			ObservedGeneration: objectSet.GetGeneration(),
			Reason:             "PartiallyPaused",
			Message:            "Waiting for ObjectSetPhases.",
		})

	case objectSet.IsPaused() && phasesArePaused:
		// Everything is paused!
		meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetPaused,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: objectSet.GetGeneration(),
			Reason:             "Paused",
			Message:            "Lifecycle state set to paused.",
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
		if k8serrors.IsNotFound(err) {
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

	done := true

	// When removing the finalizer this function may be called one last time.
	// .Teardown may allocate new watches and leave dangling watches.
	if controllerutil.ContainsFinalizer(objectSet.ClientObject(), controllers.CachedFinalizer) {
		var err error
		done, err = c.teardownHandler.Teardown(ctx, objectSet)
		if err != nil {
			return fmt.Errorf("error tearing down during deletion: %w", err)
		}
	}

	if !done {
		if objectSet.IsArchived() {
			meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
				Type:               corev1alpha1.ObjectSetArchived,
				Status:             metav1.ConditionFalse,
				Reason:             "ArchivalInProgress",
				Message:            "Object teardown in progress.",
				ObservedGeneration: objectSet.GetGeneration(),
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
			ObservedGeneration: objectSet.GetGeneration(),
		})
		objectSet.SetStatusControllerOf(nil) // we are no longer controlling anything.
	}

	return nil
}
