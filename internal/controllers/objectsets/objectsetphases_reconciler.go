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

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/ownerhandling"
	"package-operator.run/package-operator/internal/probing"
)

// objectSetPhasesReconciler reconciles all phases within an ObjectSet.
type objectSetPhasesReconciler struct {
	scheme                  *runtime.Scheme
	phaseReconciler         phaseReconciler
	remotePhase             remotePhaseReconciler
	lookupPreviousRevisions lookupPreviousRevisions
	ownerStrategy           ownerStrategy
}

type ownerStrategy interface {
	IsController(owner, obj metav1.Object) bool
}

func newObjectSetPhasesReconciler(
	scheme *runtime.Scheme,
	phaseReconciler phaseReconciler,
	remotePhase remotePhaseReconciler,
	lookupPreviousRevisions lookupPreviousRevisions,
) *objectSetPhasesReconciler {
	return &objectSetPhasesReconciler{
		scheme:                  scheme,
		phaseReconciler:         phaseReconciler,
		remotePhase:             remotePhase,
		lookupPreviousRevisions: lookupPreviousRevisions,
		ownerStrategy:           ownerhandling.NewNative(scheme),
	}
}

type remotePhaseReconciler interface {
	Reconcile(
		ctx context.Context, objectSet genericObjectSet,
		phase corev1alpha1.ObjectSetTemplatePhase,
	) ([]corev1alpha1.ControlledObjectReference, controllers.ProbingResult, error)
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
	) ([]client.Object, controllers.ProbingResult, error)

	TeardownPhase(
		ctx context.Context, owner controllers.PhaseObjectOwner,
		phase corev1alpha1.ObjectSetTemplatePhase,
	) (cleanupDone bool, err error)
}

func (r *objectSetPhasesReconciler) Reconcile(
	ctx context.Context, objectSet genericObjectSet,
) (res ctrl.Result, err error) {
	activeObjects, probingResult, err := r.reconcile(ctx, objectSet)
	if err != nil {
		return res, err
	}

	objectSet.SetStatusControllerOf(activeObjects)
	if !probingResult.IsZero() {
		meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetAvailable,
			Status:             metav1.ConditionFalse,
			Reason:             "ProbeFailure",
			Message:            probingResult.String(),
			ObservedGeneration: objectSet.ClientObject().GetGeneration(),
		})

		return res, nil
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
) ([]corev1alpha1.ControlledObjectReference, controllers.ProbingResult, error) {
	previous, err := r.lookupPreviousRevisions(ctx, objectSet)
	if err != nil {
		return nil, controllers.ProbingResult{}, fmt.Errorf("lookup previous revisions: %w", err)
	}

	probe, err := probing.Parse(
		ctx, objectSet.GetAvailabilityProbes())
	if err != nil {
		return nil, controllers.ProbingResult{}, fmt.Errorf("parsing probes: %w", err)
	}

	var activeObjects []corev1alpha1.ControlledObjectReference
	for _, phase := range objectSet.GetPhases() {
		active, probingResult, err := r.reconcilePhase(
			ctx, objectSet, phase, probe, previous)
		if err != nil {
			return nil, controllers.ProbingResult{}, err
		}

		// always gather all active objects
		activeObjects = append(activeObjects, active...)

		if !probingResult.IsZero() {
			// break on first failing probe
			return activeObjects, probingResult, nil
		}
	}

	return activeObjects, controllers.ProbingResult{}, nil
}

func (r *objectSetPhasesReconciler) reconcilePhase(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober,
	previous []controllers.PreviousObjectSet,
) ([]corev1alpha1.ControlledObjectReference, controllers.ProbingResult, error) {
	if len(phase.Class) > 0 {
		return r.remotePhase.Reconcile(
			ctx, objectSet, phase)
	}
	return r.reconcileLocalPhase(
		ctx, objectSet, phase, probe, previous)
}

// Reconciles the Phase directly in-process.
func (r *objectSetPhasesReconciler) reconcileLocalPhase(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober,
	previous []controllers.PreviousObjectSet,
) ([]corev1alpha1.ControlledObjectReference, controllers.ProbingResult, error) {
	actualObjects, probingResult, err := r.phaseReconciler.ReconcilePhase(
		ctx, objectSet, phase, probe, previous)
	if err != nil {
		return nil, probingResult, err
	}

	activeObjects, err := controllers.GetControllerOf(
		ctx, r.scheme, r.ownerStrategy,
		objectSet.ClientObject(), actualObjects)
	if err != nil {
		return nil, controllers.ProbingResult{}, err
	}
	return activeObjects, probingResult, nil
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
