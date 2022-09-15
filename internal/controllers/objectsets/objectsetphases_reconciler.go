package objectsets

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/probing"
)

// objectSetPhasesReconciler reconciles all phases within an ObjectSet.
type objectSetPhasesReconciler struct {
	phaseReconciler         phaseReconciler
	remotePhase             remotePhaseReconciler
	lookupPreviousRevisions lookupPreviousRevisions
}

func newObjectSetPhasesReconciler(
	phaseReconciler phaseReconciler,
	remotePhase remotePhaseReconciler,
	lookupPreviousRevisions lookupPreviousRevisions,
) *objectSetPhasesReconciler {
	return &objectSetPhasesReconciler{
		phaseReconciler:         phaseReconciler,
		remotePhase:             remotePhase,
		lookupPreviousRevisions: lookupPreviousRevisions,
	}
}

type remotePhaseReconciler interface {
	Reconcile(
		ctx context.Context, objectSet genericObjectSet,
		phase corev1alpha1.ObjectSetTemplatePhase,
	) (err error)
	Teardown(
		ctx context.Context, objectSet genericObjectSet,
		phase corev1alpha1.ObjectSetTemplatePhase,
	) (cleanupDone bool, err error)
}

type lookupPreviousRevisions func(
	ctx context.Context, owner controllers.PreviousOwner,
) ([]controllers.PreviousObjectSet, error)

type phaseReconciler interface {
	ReconcilePhase(
		ctx context.Context, owner controllers.PhaseObjectOwner,
		phase corev1alpha1.ObjectSetTemplatePhase,
		probe probing.Prober, previous []controllers.PreviousObjectSet,
	) (err error)

	TeardownPhase(
		ctx context.Context, owner controllers.PhaseObjectOwner,
		phase corev1alpha1.ObjectSetTemplatePhase,
	) (cleanupDone bool, err error)
}

func (r *objectSetPhasesReconciler) Reconcile(
	ctx context.Context, objectSet genericObjectSet,
) (res ctrl.Result, err error) {
	err = r.reconcile(ctx, objectSet)
	var phaseProbingFailedError *controllers.PhaseProbingFailedError
	if errors.As(err, &phaseProbingFailedError) {
		meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetAvailable,
			Status:             metav1.ConditionFalse,
			Reason:             "ProbeFailure",
			Message:            phaseProbingFailedError.Error(),
			ObservedGeneration: objectSet.ClientObject().GetGeneration(),
		})

		return res, nil
	}
	if err != nil {
		return res, err
	}

	if !meta.IsStatusConditionTrue(
		*objectSet.GetConditions(), corev1alpha1.ObjectSetSucceeded) {
		// Remember that this rollout worked!
		meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetSucceeded,
			Status:             metav1.ConditionTrue,
			Reason:             "AvailableOnce",
			Message:            "Object was available once and passed all probes.",
			ObservedGeneration: objectSet.ClientObject().GetGeneration(),
		})
	}

	meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
		Type:               corev1alpha1.ObjectSetAvailable,
		Status:             metav1.ConditionTrue,
		Reason:             "Available",
		Message:            "Object is available and passes all probes.",
		ObservedGeneration: objectSet.ClientObject().GetGeneration(),
	})

	return
}

func (r *objectSetPhasesReconciler) reconcile(
	ctx context.Context, objectSet genericObjectSet,
) error {
	previous, err := r.lookupPreviousRevisions(ctx, objectSet)
	if err != nil {
		return fmt.Errorf("lookup previous revisions: %w", err)
	}

	probe, err := probing.Parse(
		ctx, objectSet.GetAvailabilityProbes())
	if err != nil {
		return fmt.Errorf("parsing probes: %w", err)
	}

	for _, phase := range objectSet.GetPhases() {
		if err := r.reconcilePhase(
			ctx, objectSet, phase, probe, previous); err != nil {
			return err
		}
	}

	return nil
}

func (r *objectSetPhasesReconciler) reconcilePhase(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober,
	previous []controllers.PreviousObjectSet,
) (err error) {
	if len(phase.Class) > 0 {
		err = r.remotePhase.Reconcile(
			ctx, objectSet, phase)
	} else {
		err = r.reconcileLocalPhase(
			ctx, objectSet, phase, probe, previous)
	}
	return
}

// Reconciles the Phase directly in-process.
func (r *objectSetPhasesReconciler) reconcileLocalPhase(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober,
	previous []controllers.PreviousObjectSet,
) error {
	return r.phaseReconciler.ReconcilePhase(
		ctx, objectSet, phase, probe, previous)
}

func (r *objectSetPhasesReconciler) Teardown(
	ctx context.Context, objectSet genericObjectSet,
) (cleanupDone bool, err error) {
	log := logr.FromContextOrDiscard(ctx)

	phases := objectSet.GetPhases()
	reverse(phases) // teardown in reverse order

	for _, phase := range phases {
		if cleanupDone, err := r.teardownPhase(ctx, objectSet, phase); err != nil {
			return false, fmt.Errorf("error archiving phase: %w", err)
		} else if !cleanupDone {
			return false, nil
		}
		log.Info("cleanup done", "phase", phase.Name)
	}

	return true, nil
}

func (r *objectSetPhasesReconciler) teardownPhase(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	if len(phase.Class) > 0 {
		return r.remotePhase.Teardown(ctx, objectSet, phase)
	}
	return r.phaseReconciler.TeardownPhase(ctx, objectSet, phase)
}

// reverse the order of a slice.
func reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
