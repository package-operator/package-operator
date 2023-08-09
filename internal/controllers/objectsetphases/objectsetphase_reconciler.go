package objectsetphases

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/flowcontrol"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/probing"
)

// objectSetPhaseReconciler reconciles objects within a phase.
type objectSetPhaseReconciler struct {
	scheme                  *runtime.Scheme
	phaseReconciler         phaseReconciler
	lookupPreviousRevisions lookupPreviousRevisions
	ownerStrategy           ownerStrategy
	backoff                 *flowcontrol.Backoff
}

func newObjectSetPhaseReconciler(
	scheme *runtime.Scheme,
	phaseReconciler phaseReconciler,
	lookupPreviousRevisions lookupPreviousRevisions,
	ownerStrategy ownerStrategy,
	opts ...objectSetPhaseReconcilerOption,
) *objectSetPhaseReconciler {
	var cfg objectSetPhaseReconcilerConfig

	cfg.Option(opts...)
	cfg.Default()

	return &objectSetPhaseReconciler{
		scheme:                  scheme,
		phaseReconciler:         phaseReconciler,
		lookupPreviousRevisions: lookupPreviousRevisions,
		ownerStrategy:           ownerStrategy,
		backoff:                 cfg.GetBackoff(),
	}
}

type phaseReconciler interface {
	ReconcilePhase(
		ctx context.Context, owner controllers.PhaseObjectOwner,
		phase corev1alpha1.ObjectSetTemplatePhase,
		probe probing.Prober, previous []controllers.PreviousObjectSet,
	) ([]client.Object, controllers.ProbingResult, error)

	TeardownPhase(
		ctx context.Context, owner controllers.PhaseObjectOwner,
		phase corev1alpha1.ObjectSetTemplatePhase,
	) (cleanupDone bool, err error)
}

type lookupPreviousRevisions func(
	ctx context.Context, owner controllers.PreviousOwner,
) ([]controllers.PreviousObjectSet, error)

func (r *objectSetPhaseReconciler) Reconcile(
	ctx context.Context, objectSetPhase genericObjectSetPhase,
) (res ctrl.Result, err error) {
	defer r.backoff.GC()

	controllers.DeleteMappedConditions(ctx, objectSetPhase.GetConditions())

	previous, err := r.lookupPreviousRevisions(ctx, objectSetPhase)
	if err != nil {
		return res, fmt.Errorf("lookup previous revisions: %w", err)
	}

	probe, err := probing.Parse(
		ctx, objectSetPhase.GetAvailabilityProbes())
	if err != nil {
		return res, fmt.Errorf("parsing probes: %w", err)
	}

	actualObjects, probingResult, err := r.phaseReconciler.ReconcilePhase(
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
			objectSetPhase.GetConditions(), metav1.Condition{
				Type:               corev1alpha1.ObjectSetAvailable,
				Status:             metav1.ConditionFalse,
				Reason:             "ProbeFailure",
				Message:            probingResult.StringWithoutPhase(),
				ObservedGeneration: objectSetPhase.ClientObject().GetGeneration(),
			})

		return res, nil
	}

	meta.SetStatusCondition(objectSetPhase.GetConditions(), metav1.Condition{
		Type:               corev1alpha1.ObjectSetPhaseAvailable,
		Status:             metav1.ConditionTrue,
		Reason:             "Available",
		Message:            "Object is available and passes all probes.",
		ObservedGeneration: objectSetPhase.ClientObject().GetGeneration(),
	})

	return ctrl.Result{}, nil
}

func (r *objectSetPhaseReconciler) Teardown(
	ctx context.Context, objectSetPhase genericObjectSetPhase,
) (cleanupDone bool, err error) {
	return r.phaseReconciler.TeardownPhase(
		ctx, objectSetPhase, objectSetPhase.GetPhase())
}

// Sets .status.activeObjects to all objects actively reconciled and controlled by this Phase.
func (r *objectSetPhaseReconciler) reportOwnActiveObjects(
	ctx context.Context, objectSetPhase genericObjectSetPhase, actualObjects []client.Object,
) error {
	activeObjects, err := controllers.GetControllerOf(
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
