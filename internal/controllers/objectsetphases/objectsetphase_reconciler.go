package objectsetphases

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/probing"
)

// objectSetPhaseReconciler reconciles objects within a phase.
type objectSetPhaseReconciler struct {
	scheme                  *runtime.Scheme
	phaseReconciler         phaseReconciler
	lookupPreviousRevisions lookupPreviousRevisions
	ownerStrategy           ownerStrategy
}

func newObjectSetPhaseReconciler(
	scheme *runtime.Scheme,
	phaseReconciler phaseReconciler,
	lookupPreviousRevisions lookupPreviousRevisions,
	ownerStrategy ownerStrategy,
) *objectSetPhaseReconciler {
	return &objectSetPhaseReconciler{
		scheme:                  scheme,
		phaseReconciler:         phaseReconciler,
		lookupPreviousRevisions: lookupPreviousRevisions,
		ownerStrategy:           ownerStrategy,
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
	if err != nil {
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
	activeObjects, err := controllers.FilterOwnActiveObjects(
		ctx, r.scheme, r.ownerStrategy,
		objectSetPhase.ClientObject(), actualObjects)
	if err != nil {
		return err
	}
	objectSetPhase.SetStatusActiveObjects(activeObjects)
	return nil
}
