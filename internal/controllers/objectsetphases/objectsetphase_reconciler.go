package objectsetphases

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/probing"
)

// objectSetPhaseReconciler reconciles objects within a phase.
type objectSetPhaseReconciler struct {
	phaseReconciler         phaseReconciler
	lookupPreviousRevisions lookupPreviousRevisions
}

func newObjectSetPhaseReconciler(
	phaseReconciler phaseReconciler,
	lookupPreviousRevisions lookupPreviousRevisions,
) *objectSetPhaseReconciler {
	return &objectSetPhaseReconciler{
		phaseReconciler:         phaseReconciler,
		lookupPreviousRevisions: lookupPreviousRevisions,
	}
}

type phaseReconciler interface {
	ReconcilePhase(
		ctx context.Context, owner controllers.PhaseObjectOwner,
		phase corev1alpha1.ObjectSetTemplatePhase,
		probe probing.Prober, previous []controllers.PreviousObjectSet,
	) error

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
	previous, err := r.lookupPreviousRevisions(ctx, objectSetPhase)
	if err != nil {
		return res, fmt.Errorf("lookup previous revisions: %w", err)
	}

	probe, err := probing.Parse(
		ctx, objectSetPhase.GetAvailabilityProbes())
	if err != nil {
		return res, fmt.Errorf("parsing probes: %w", err)
	}

	err = r.phaseReconciler.ReconcilePhase(
		ctx, objectSetPhase, objectSetPhase.GetPhase(), probe, previous)
	var phaseProbingFailedError *controllers.PhaseProbingFailedError
	if errors.As(err, &phaseProbingFailedError) {
		meta.SetStatusCondition(
			objectSetPhase.GetConditions(), metav1.Condition{
				Type:               corev1alpha1.ObjectSetAvailable,
				Status:             metav1.ConditionFalse,
				Reason:             "ProbeFailure",
				Message:            phaseProbingFailedError.ErrorWithoutPhase(),
				ObservedGeneration: objectSetPhase.ClientObject().GetGeneration(),
			})

		return res, nil
	}
	if err != nil {
		return res, err
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
