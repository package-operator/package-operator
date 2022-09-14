package objectsets

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/probing"
)

// phasesReconciler reconciles all phases within an ObjectSet.
type phasesReconciler struct {
	client          client.Client
	phaseReconciler phaseReconciler
	scheme          *runtime.Scheme
	newObjectSet    genericObjectSetFactory
}

func newPhasesReconciler(
	client client.Client,
	phaseReconciler phaseReconciler,
	scheme *runtime.Scheme,
	newObjectSet genericObjectSetFactory,
) *phasesReconciler {
	return &phasesReconciler{
		client:          client,
		phaseReconciler: phaseReconciler,
		scheme:          scheme,
		newObjectSet:    newObjectSet,
	}
}

type phaseReconciler interface {
	ReconcilePhase(
		ctx context.Context, owner controllers.PhaseObjectOwner,
		phase corev1alpha1.ObjectSetTemplatePhase,
		probe probing.Prober, previous []client.Object,
	) (failedProbes []string, err error)

	TeardownPhase(
		ctx context.Context, owner client.Object,
		phase corev1alpha1.ObjectSetTemplatePhase,
	) (cleanupDone bool, err error)
}

func (r *phasesReconciler) Reconcile(
	ctx context.Context, objectSet genericObjectSet,
) (res ctrl.Result, err error) {
	previous, err := r.lookupPreviousRevisions(ctx, objectSet)
	if err != nil {
		return res, fmt.Errorf("lookup previous revisions: %w", err)
	}

	probe, err := probing.Parse(
		ctx, objectSet.GetAvailabilityProbes())
	if err != nil {
		return res, fmt.Errorf("parsing probes: %w", err)
	}
	for _, phase := range objectSet.GetPhases() {
		var (
			failedProbes []string
			err          error
		)
		if len(phase.Class) > 0 {
			failedProbes, err = r.reconcileRemotePhase(
				ctx, objectSet, phase)
		} else {
			failedProbes, err = r.reconcileLocalPhase(
				ctx, objectSet, phase, probe, previous)
		}
		if err != nil {
			return ctrl.Result{}, err
		}

		if len(failedProbes) > 0 {
			meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
				Type:               corev1alpha1.ObjectSetAvailable,
				Status:             metav1.ConditionFalse,
				Reason:             "ProbeFailure",
				Message:            fmt.Sprintf("Phase %q failed: %s", phase.Name, strings.Join(failedProbes, ", ")),
				ObservedGeneration: objectSet.ClientObject().GetGeneration(),
			})
			return ctrl.Result{}, nil
		}
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

// Reconciles the Phase via an ObjectSetPhase object,
// delegating the task to an auxiliary controller.
func (r *phasesReconciler) reconcileRemotePhase(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (failedProbes []string, err error) {
	// TODO!
	return
}

// Reconciles the Phase directly in-process.
func (r *phasesReconciler) reconcileLocalPhase(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober, previous []client.Object,
) ([]string, error) {
	return r.phaseReconciler.ReconcilePhase(
		ctx, objectSet, phase, probe, previous)
}

func (r *phasesReconciler) lookupPreviousRevisions(
	ctx context.Context, objectSet genericObjectSet,
) ([]client.Object, error) {
	previous := objectSet.GetPrevious()
	previousSets := make([]client.Object, len(previous))
	for i, prev := range objectSet.GetPrevious() {
		set := r.newObjectSet(r.scheme)
		if err := r.client.Get(
			ctx, client.ObjectKey{
				Name: prev.Name, Namespace: objectSet.ClientObject().GetNamespace(),
			}, set.ClientObject()); err != nil {
			return nil, err
		}
		previousSets[i] = set.ClientObject()
	}
	return previousSets, nil
}

func (r *phasesReconciler) Teardown(
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

func (r *phasesReconciler) teardownPhase(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	if len(phase.Class) > 0 {
		return r.teardownRemotePhase(ctx, objectSet, phase)
	}
	return r.phaseReconciler.TeardownPhase(
		ctx, objectSet.ClientObject(), phase)
}

func (r *phasesReconciler) teardownRemotePhase(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	// TODO!
	return true, nil
}

// reverse the order of a slice.
func reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
