package objectsetphases

import (
	"context"
	"fmt"

	"pkg.package-operator.run/boxcutter/managedcache"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/flowcontrol"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/controllers"
	internalprobing "package-operator.run/internal/probing"
)

// objectSetPhaseReconciler reconciles objects within a phase.
type objectSetPhaseReconciler struct {
	scheme                  *runtime.Scheme
	accessManager           managedcache.ObjectBoundAccessManager[client.Object]
	phaseReconcilerFactory  controllers.PhaseReconcilerFactory
	lookupPreviousRevisions lookupPreviousRevisions
	ownerStrategy           ownerStrategy
	backoff                 *flowcontrol.Backoff
}

func newObjectSetPhaseReconciler(
	scheme *runtime.Scheme,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	phaseReconcilerFactory controllers.PhaseReconcilerFactory,
	lookupPreviousRevisions lookupPreviousRevisions,
	ownerStrategy ownerStrategy,
) *objectSetPhaseReconciler {
	var cfg objectSetPhaseReconcilerConfig

	cfg.Default()

	return &objectSetPhaseReconciler{
		scheme:                  scheme,
		accessManager:           accessManager,
		phaseReconcilerFactory:  phaseReconcilerFactory,
		lookupPreviousRevisions: lookupPreviousRevisions,
		ownerStrategy:           ownerStrategy,
		backoff:                 cfg.GetBackoff(),
	}
}

type lookupPreviousRevisions func(
	ctx context.Context, owner controllers.PreviousOwner,
) ([]controllers.PreviousObjectSet, error)

func (r *objectSetPhaseReconciler) Reconcile(
	ctx context.Context, objectSetPhase adapters.ObjectSetPhaseAccessor,
) (res ctrl.Result, err error) {
	defer r.backoff.GC()

	controllers.DeleteMappedConditions(ctx, objectSetPhase.GetStatusConditions())

	previous, err := r.lookupPreviousRevisions(ctx, objectSetPhase)
	if err != nil {
		return res, fmt.Errorf("lookup previous revisions: %w", err)
	}

	probe, err := internalprobing.Parse(
		ctx, objectSetPhase.GetAvailabilityProbes())
	if err != nil {
		return res, fmt.Errorf("parsing probes: %w", err)
	}

	objectsInPhase := []client.Object{}
	for _, object := range objectSetPhase.GetPhase().Objects {
		objectsInPhase = append(objectsInPhase, &object.Object)
	}
	cache, err := r.accessManager.GetWithUser(
		ctx,
		constants.StaticCacheOwner(),
		objectSetPhase.ClientObject(),
		objectsInPhase,
	)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("preparing cache: %w", err)
	}
	phaseReconciler := r.phaseReconcilerFactory.New(cache)

	actualObjects, probingResult, err := phaseReconciler.ReconcilePhase(
		ctx, objectSetPhase, objectSetPhase.GetPhase(), probe, previous)
	if controllers.IsExternalResourceNotFound(err) {
		id := string(objectSetPhase.ClientObject().GetUID())

		r.backoff.Next(id, r.backoff.Clock.Now())

		return ctrl.Result{
			RequeueAfter: r.backoff.Get(id),
		}, nil
	} else if err != nil {
		return res, err
	}

	if err := r.reportOwnActiveObjects(ctx, objectSetPhase, actualObjects); err != nil {
		return res, fmt.Errorf("reporting active objects: %w", err)
	}

	if !probingResult.IsZero() {
		meta.SetStatusCondition(
			objectSetPhase.GetStatusConditions(), metav1.Condition{
				Type:               corev1alpha1.ObjectSetAvailable,
				Status:             metav1.ConditionFalse,
				Reason:             "ProbeFailure",
				Message:            probingResult.StringWithoutPhase(),
				ObservedGeneration: objectSetPhase.ClientObject().GetGeneration(),
			})

		return res, nil
	}

	meta.SetStatusCondition(objectSetPhase.GetStatusConditions(), metav1.Condition{
		Type:               corev1alpha1.ObjectSetPhaseAvailable,
		Status:             metav1.ConditionTrue,
		Reason:             "Available",
		Message:            "Object is available and passes all probes.",
		ObservedGeneration: objectSetPhase.ClientObject().GetGeneration(),
	})

	return ctrl.Result{}, nil
}

func (r *objectSetPhaseReconciler) Teardown(
	ctx context.Context, objectSetPhase adapters.ObjectSetPhaseAccessor,
) (cleanupDone bool, err error) {
	// objectSetPhase is deleted with the `orphan` cascade option, so we don't need to delete the owned objects.
	if controllerutil.ContainsFinalizer(objectSetPhase.ClientObject(), "orphan") {
		return true, nil
	}

	objectsInPhase := []client.Object{}
	for _, object := range objectSetPhase.GetPhase().Objects {
		objectsInPhase = append(objectsInPhase, &object.Object)
	}
	cache, err := r.accessManager.GetWithUser(
		ctx,
		constants.StaticCacheOwner(),
		objectSetPhase.ClientObject(),
		objectsInPhase,
	)
	if err != nil {
		return false, fmt.Errorf("preparing cache: %w", err)
	}
	phaseReconciler := r.phaseReconcilerFactory.New(cache)

	cleanupDone, err = phaseReconciler.TeardownPhase(
		ctx, objectSetPhase, objectSetPhase.GetPhase())
	if err != nil {
		return false, err
	}
	if !cleanupDone {
		return false, nil
	}

	if err := r.accessManager.FreeWithUser(
		ctx,
		constants.StaticCacheOwner(),
		objectSetPhase.ClientObject(),
	); err != nil {
		return false, fmt.Errorf("freewithuser: %w", err)
	}

	return true, nil
}

// Sets .status.activeObjects to all objects actively reconciled and controlled by this Phase.
func (r *objectSetPhaseReconciler) reportOwnActiveObjects(
	ctx context.Context, objectSetPhase adapters.ObjectSetPhaseAccessor, actualObjects []client.Object,
) error {
	activeObjects, err := controllers.GetStatusControllerOf(
		ctx, r.scheme, r.ownerStrategy,
		objectSetPhase.ClientObject(), actualObjects)
	if err != nil {
		return err
	}
	objectSetPhase.SetStatusControllerOf(activeObjects)
	return nil
}

type objectSetPhaseReconcilerConfig struct {
	controllers.BackoffConfig
}

func (c *objectSetPhaseReconcilerConfig) Option(opts ...objectSetPhaseReconcilerOption) {
	for _, opt := range opts {
		opt.ConfigureObjectSetPhaseReconciler(c)
	}
}

func (c *objectSetPhaseReconcilerConfig) Default() {
	c.BackoffConfig.Default()
}

type objectSetPhaseReconcilerOption interface {
	ConfigureObjectSetPhaseReconciler(*objectSetPhaseReconcilerConfig)
}
