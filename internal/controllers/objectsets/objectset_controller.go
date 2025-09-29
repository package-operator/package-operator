package objectsets

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/metrics"
	"package-operator.run/internal/preflight"

	"pkg.package-operator.run/boxcutter/managedcache"
	"pkg.package-operator.run/boxcutter/ownerhandling"
)

// Generic reconciler for both ObjectSet and ClusterObjectSet objects.
type GenericObjectSetController struct {
	newObjectSet      adapters.ObjectSetAccessorFactory
	newObjectSetPhase adapters.ObjectSetPhaseFactory

	client     client.Client
	log        logr.Logger
	scheme     *runtime.Scheme
	reconciler []reconciler

	recorder        metricsRecorder
	accessManager   managedcache.ObjectBoundAccessManager[client.Object]
	teardownHandler teardownHandler
}

type reconciler interface {
	Reconcile(ctx context.Context, objectSet adapters.ObjectSetAccessor) (ctrl.Result, error)
}

type teardownHandler interface {
	Teardown(
		ctx context.Context, objectSet adapters.ObjectSetAccessor,
	) (cleanupDone bool, err error)
}

type metricsRecorder interface {
	RecordObjectSetMetrics(objectSet metrics.GenericObjectSet)
}

func NewObjectSetController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	accessManager managedcache.ObjectBoundAccessManager[client.Object], uc client.Reader,
	r metricsRecorder, restMapper meta.RESTMapper,
) *GenericObjectSetController {
	return newGenericObjectSetController(
		adapters.NewObjectSet,
		adapters.NewObjectSetPhaseAccessor,
		adapters.NewObjectSlice,
		c, log, scheme, accessManager, uc, r,
		restMapper,
	)
}

func NewClusterObjectSetController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	accessManager managedcache.ObjectBoundAccessManager[client.Object], uc client.Reader,
	r metricsRecorder, restMapper meta.RESTMapper,
) *GenericObjectSetController {
	return newGenericObjectSetController(
		adapters.NewClusterObjectSet,
		adapters.NewClusterObjectSetPhaseAccessor,
		adapters.NewClusterObjectSlice,
		c, log, scheme, accessManager, uc, r,
		restMapper,
	)
}

func newGenericObjectSetController(
	newObjectSet adapters.ObjectSetAccessorFactory,
	newObjectSetPhase adapters.ObjectSetPhaseFactory,
	newObjectSlice adapters.ObjectSliceFactory,
	client client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	accessManager managedcache.ObjectBoundAccessManager[client.Object], uncachedClient client.Reader,
	recorder metricsRecorder, restMapper meta.RESTMapper,
) *GenericObjectSetController {
	controller := &GenericObjectSetController{
		newObjectSet:      newObjectSet,
		newObjectSetPhase: newObjectSetPhase,

		client:        client,
		log:           log,
		scheme:        scheme,
		accessManager: accessManager,
		recorder:      recorder,
	}

	phasesReconciler := newObjectSetPhasesReconciler(
		scheme,
		accessManager,
		controllers.NewPhaseReconcilerFactory(
			scheme,
			uncachedClient,
			ownerhandling.NewNative(scheme),
			preflight.NewAPIExistence(restMapper,
				preflight.List{
					preflight.NewNoOwnerReferences(restMapper),
					preflight.NewNamespaceEscalation(restMapper),
					preflight.NewDryRun(client),
				},
			),
		),
		newObjectSetRemotePhaseReconciler(
			client, uncachedClient, scheme, newObjectSetPhase),
		controllers.NewPreviousRevisionLookup(
			scheme, func(s *runtime.Scheme) controllers.PreviousObjectSet {
				return newObjectSet(s)
			}, client).Lookup,
		preflight.PhasesCheckerList{
			preflight.NewObjectDuplicate(),
		},
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
		For(objectSet, builder.WithPredicates(
			&predicate.GenerationChangedPredicate{},
			predicate.NewPredicateFuncs(func(object client.Object) bool {
				c.log.Info(
					"processing cache event",
					"gvk", object.GetObjectKind().GroupVersionKind(),
					"object", client.ObjectKeyFromObject(object),
				)
				return true
			}),
		)).
		Owns(objectSetPhase).
		WatchesRawSource(
			c.accessManager.Source(
				handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), objectSet),
				predicate.NewPredicateFuncs(func(object client.Object) bool {
					c.log.V(constants.LogLevelDebug).Info(
						"processing dynamic cache event",
						"gvk", object.GetObjectKind().GroupVersionKind(),
						"object", client.ObjectKeyFromObject(object),
						"owners", object.GetOwnerReferences(),
					)
					return true
				}),
			),
		).
		Complete(c)
}

func (c *GenericObjectSetController) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	log := c.log.WithValues("ObjectSet", req.String())
	log.Info("reconcile")
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
		// Add the metrics finalizer if the object doesn't have a deletion timestamp
		if objectSet.ClientObject().GetDeletionTimestamp().IsZero() {
			if err = controllers.EnsureFinalizer(ctx, c.client,
				objectSet.ClientObject(), constants.MetricsFinalizer); err != nil {
				return
			}
		}
		if c.recorder != nil {
			c.recorder.RecordObjectSetMetrics(objectSet)
		}
		// If the objectset has a deletion timestamp, remove the metrics finalizer
		if !objectSet.ClientObject().GetDeletionTimestamp().IsZero() {
			err = client.IgnoreNotFound(controllers.RemoveFinalizer(
				ctx, c.client, objectSet.ClientObject(), constants.MetricsFinalizer))
		}
	}()

	if objectSet.GetStatusRevision() != 0 && objectSet.GetSpecRevision() == 0 { //nolint:staticcheck
		// Update existing ObjectSets to include .spec.revision
		// to phase in new revision numbering approach.
		objectSet.SetSpecRevision(objectSet.GetStatusRevision()) //nolint:staticcheck
		if err = c.client.Update(ctx, objectSet.ClientObject()); err != nil {
			return res, fmt.Errorf("update revision in spec: %w", err)
		}
	}

	if meta.IsStatusConditionTrue(*objectSet.GetStatusConditions(), corev1alpha1.ObjectSetArchived) {
		// We don't want to touch this object anymore.
		return res, nil
	}

	if !objectSet.ClientObject().GetDeletionTimestamp().IsZero() ||
		objectSet.IsSpecArchived() {
		if err := c.handleDeletionAndArchival(ctx, objectSet); err != nil {
			return res, err
		}

		// only try to update status if the object was archived, not if it was deleted
		if objectSet.ClientObject().GetDeletionTimestamp().IsZero() {
			err = c.updateStatus(ctx, objectSet)
		}
		return res, err
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
		return controllers.UpdateObjectSetOrPhaseStatusFromError(ctx, objectSet, err,
			func(ctx context.Context) error {
				return c.updateStatus(ctx, objectSet)
			})
	}

	if err := c.reportPausedCondition(ctx, objectSet); err != nil {
		return res, fmt.Errorf("getting paused status: %w", err)
	}

	return res, c.updateStatus(ctx, objectSet)
}

func (c *GenericObjectSetController) updateStatus(ctx context.Context, objectSet adapters.ObjectSetAccessor) error {
	if err := c.client.Status().Update(ctx, objectSet.ClientObject()); err != nil {
		return fmt.Errorf("updating ObjectSet status: %w", err)
	}
	return nil
}

func (c *GenericObjectSetController) reportPausedCondition(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
) error {
	var phasesArePaused, unknown bool
	if len(objectSet.GetStatusRemotePhases()) > 0 {
		var err error
		phasesArePaused, unknown, err = c.areRemotePhasesPaused(ctx, objectSet)
		if err != nil {
			return fmt.Errorf("getting status of remote phases: %w", err)
		}
	} else {
		phasesArePaused = objectSet.IsSpecPaused()
	}

	switch {
	case unknown ||
		objectSet.IsSpecPaused() && !phasesArePaused ||
		!objectSet.IsSpecPaused() && phasesArePaused:
		// Could not get status of all remote ObjectSetPhases or they disagree with their parent.
		meta.SetStatusCondition(objectSet.GetStatusConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetPaused,
			Status:             metav1.ConditionUnknown,
			ObservedGeneration: objectSet.ClientObject().GetGeneration(),
			Reason:             "PartiallyPaused",
			Message:            "Waiting for ObjectSetPhases.",
		})

	case objectSet.IsSpecPaused() && phasesArePaused:
		// Everything is paused!
		meta.SetStatusCondition(objectSet.GetStatusConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetPaused,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: objectSet.ClientObject().GetGeneration(),
			Reason:             "Paused",
			Message:            "Lifecycle state set to paused.",
		})

	case !objectSet.IsSpecPaused() && !phasesArePaused:
		// Nothing is paused!
		meta.RemoveStatusCondition(objectSet.GetStatusConditions(), corev1alpha1.ObjectSetPaused)
	}
	return nil
}

func (c *GenericObjectSetController) areRemotePhasesPaused(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
) (arePaused, unknown bool, err error) {
	var pausedPhases int
	for _, phaseRef := range objectSet.GetStatusRemotePhases() {
		phase := c.newObjectSetPhase(c.scheme)
		err := c.client.Get(ctx, client.ObjectKey{
			Name:      phaseRef.Name,
			Namespace: objectSet.ClientObject().GetNamespace(),
		}, phase.ClientObject())
		if apimachineryerrors.IsNotFound(err) {
			// Phase object is not yet in cache, or was deleted by someone else.
			// -> we have to wait, but we don't want to raise an error in logs.
			return false, true, nil
		}
		if err != nil {
			return false, false, fmt.Errorf("get ObjectSetPhase: %w", err)
		}

		if meta.IsStatusConditionTrue(*phase.GetStatusConditions(), corev1alpha1.ObjectSetPhasePaused) {
			pausedPhases++
		}
	}
	arePaused = pausedPhases == len(objectSet.GetStatusRemotePhases())
	return arePaused, false, nil
}

func (c *GenericObjectSetController) handleDeletionAndArchival(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
) error {
	// always make sure to remove Available condition
	defer meta.RemoveStatusCondition(objectSet.GetStatusConditions(), corev1alpha1.ObjectSetAvailable)

	done := true

	// When removing the finalizer this function may be called one last time.
	// .Teardown may allocate new watches and leave dangling watches.
	if controllerutil.ContainsFinalizer(objectSet.ClientObject(), constants.CachedFinalizer) {
		var err error
		done, err = c.teardownHandler.Teardown(ctx, objectSet)
		if err != nil {
			return fmt.Errorf("error tearing down during deletion: %w", err)
		}
	}

	if !done {
		if objectSet.IsSpecArchived() {
			meta.SetStatusCondition(objectSet.GetStatusConditions(), metav1.Condition{
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

	if err := c.accessManager.FreeWithUser(ctx, constants.StaticCacheOwner(), objectSet.ClientObject()); err != nil {
		return fmt.Errorf("freeing cache: %w", err)
	}

	if err := controllers.RemoveCacheFinalizer(
		ctx, c.client, objectSet.ClientObject()); err != nil {
		return err
	}

	// Needs to be called _after_ FreeCacheAndRemoveFinalizer,
	// because .Update is loading new state into objectSet, overriding changes to conditions.
	if objectSet.IsSpecArchived() {
		meta.SetStatusCondition(objectSet.GetStatusConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetArchived,
			Status:             metav1.ConditionTrue,
			Reason:             "Archived",
			ObservedGeneration: objectSet.ClientObject().GetGeneration(),
		})
		objectSet.SetStatusControllerOf(nil) // we are no longer controlling anything.
	}

	return nil
}
