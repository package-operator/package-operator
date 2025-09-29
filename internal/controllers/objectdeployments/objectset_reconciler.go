package objectdeployments

import (
	"context"
	"fmt"

	"package-operator.run/internal/adapters"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/controllers"
)

type objectSetReconciler struct {
	client                      client.Client
	listObjectSetsForDeployment listObjectSetsForDeploymentFn
	reconcilers                 []objectSetSubReconciler
}

type objectSetSubReconciler interface {
	Reconcile(
		ctx context.Context, currentObjectSet adapters.ObjectSetAccessor,
		prevObjectSets []adapters.ObjectSetAccessor, objectDeployment adapters.ObjectDeploymentAccessor,
	) (ctrl.Result, error)
}

type listObjectSetsForDeploymentFn func(
	ctx context.Context, objectDeployment adapters.ObjectDeploymentAccessor,
) ([]adapters.ObjectSetAccessor, error)

func (o *objectSetReconciler) Reconcile(
	ctx context.Context, objectDeployment adapters.ObjectDeploymentAccessor,
) (ctrl.Result, error) {
	objectSets, err := o.listObjectSetsForDeployment(ctx, objectDeployment)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("listing objectsets under deployment errored: %w", err)
	}

	// Delay any action until all ObjectSets under management report .status.revision
	for _, objectSet := range objectSets {
		if objectSet.GetStatusRevision() == 0 || objectSet.GetSpecRevision() == 0 { //nolint:staticcheck
			return ctrl.Result{}, nil
		}
	}

	// objectSets is already sorted ascending by .spec.revision
	// check if the latest revision is up-to-date, by comparing their hash.
	var (
		currentObjectSet adapters.ObjectSetAccessor
		prevObjectSets   []adapters.ObjectSetAccessor
	)
	if len(objectSets) > 0 {
		maybeCurrentObjectSet := objectSets[len(objectSets)-1]
		annotations := maybeCurrentObjectSet.ClientObject().GetAnnotations()
		if annotations != nil {
			if hash, ok := annotations[ObjectSetHashAnnotation]; ok &&
				hash == objectDeployment.GetStatusTemplateHash() {
				currentObjectSet = maybeCurrentObjectSet
				prevObjectSets = objectSets[0 : len(objectSets)-1] // previous is everything excluding current
			}
		}
	}
	if currentObjectSet == nil {
		// all ObjectSets are outdated.
		prevObjectSets = objectSets
	}

	for _, objectSet := range objectSets {
		if objectSet.IsSpecArchived() {
			continue
		}

		// The pause value in the ObjectDeployment controls the pause value in ObjectSet.
		// Update only when the values differ.
		if objectDeployment.GetSpecPaused() != objectSet.GetSpecPausedByParent() {
			var pauseChangeMsg string
			if objectDeployment.GetSpecPaused() {
				objectSet.SetSpecPausedByParent()
				pauseChangeMsg = "pause"
			} else {
				objectSet.SetSpecActiveByParent()
				pauseChangeMsg = "unpause"
			}

			if err = o.client.Update(ctx, objectSet.ClientObject()); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to %s objectset: %w", pauseChangeMsg, err)
			}
		}
	}

	// Skip subreconcilers when paused
	if objectDeployment.GetSpecPaused() {
		o.setObjectDeploymentStatus(ctx, currentObjectSet, prevObjectSets, objectDeployment)
		return ctrl.Result{}, nil
	}

	var (
		res              ctrl.Result
		subReconcilerErr error
	)

	for _, reconciler := range o.reconcilers {
		res, subReconcilerErr = reconciler.Reconcile(ctx, currentObjectSet, prevObjectSets, objectDeployment)
		if subReconcilerErr != nil || !res.IsZero() {
			break
		}
	}

	if subReconcilerErr != nil || !res.IsZero() {
		return res, subReconcilerErr
	}
	o.setObjectDeploymentStatus(ctx, currentObjectSet, prevObjectSets, objectDeployment)
	return ctrl.Result{}, nil
}

// Does current objectset exist?
// N -> ObjectDeployment Progressing = True / Is a previous objectset available?
// __Y -> ObjectDeployment Available = True
// __N -> ObjectDeployment Available = False
// Y -> Is current objectset successful?
// __N -> ObjectDeployment Progressing = True / Is a previous objectset available?
// ____Y -> ObjectDeployment Available = True
// ____N -> ObjectDeployment Available = False
// __Y -> ObjectDeployment Progressing = False / Is current objectset available?
// ____N -> Is a previous objectset available?
// ______Y -> ObjectDeployment Available = True
// ______N -> ObjectDeployment Available = False
// ____Y -> ObjectDeployment Available = True.
func (o *objectSetReconciler) setObjectDeploymentStatus(ctx context.Context,
	currentObjectSet adapters.ObjectSetAccessor,
	prevObjectSets []adapters.ObjectSetAccessor,
	objectDeployment adapters.ObjectDeploymentAccessor,
) {
	if currentObjectSet == nil {
		objectDeployment.SetStatusConditions(
			newProgressingCondition(
				metav1.ConditionTrue,
				progressingReasonProgressing,
				"Progressing to a new ObjectSet.",
				objectDeployment.ClientObject().GetGeneration(),
			),
			conditionFromPreviousObjectSets(objectDeployment.GetGeneration(), prevObjectSets...),
		)
		if len(prevObjectSets) > 0 {
			objectDeployment.SetStatusRevision(prevObjectSets[0].GetSpecRevision())
		}
		return
	}

	objectDeployment.SetStatusRevision(currentObjectSet.GetSpecRevision())

	// map conditions
	// -> copy mapped status conditions
	controllers.DeleteMappedConditions(ctx, objectDeployment.GetStatusConditions())
	controllers.MapConditions(
		ctx,
		currentObjectSet.ClientObject().GetGeneration(), *currentObjectSet.GetStatusConditions(),
		objectDeployment.ClientObject().GetGeneration(), objectDeployment.GetStatusConditions(),
	)

	if !meta.IsStatusConditionTrue(*currentObjectSet.GetStatusConditions(), corev1alpha1.ObjectSetSucceeded) {
		var conds []metav1.Condition

		msg := "Latest Revision Status Unknown"

		availableCond := meta.FindStatusCondition(*currentObjectSet.GetStatusConditions(), corev1alpha1.ObjectSetAvailable)
		if availableCond != nil {
			if availableCond.Status == metav1.ConditionFalse {
				conds = append(conds, conditionFromPreviousObjectSets(objectDeployment.GetGeneration(), prevObjectSets...))

				msg = "Latest Revision is Unavailable: " + availableCond.Message
			} else {
				msg = "Latest Revision is Available: pending success delay period"
			}
		}

		conds = append(conds, newProgressingCondition(
			metav1.ConditionTrue,
			progressingReasonLatestRevPendingSuccess,
			msg,
			objectDeployment.ClientObject().GetGeneration(),
		))

		objectDeployment.SetStatusConditions(conds...)

		return
	}

	// Latest revision succeeded, so we are no longer progressing.
	objectDeployment.SetStatusConditions(
		newProgressingCondition(
			metav1.ConditionFalse,
			progressingReasonIdle,
			"Update concluded.",
			objectDeployment.GetGeneration(),
		),
	)

	if !currentObjectSet.IsSpecAvailable() {
		objectDeployment.SetStatusConditions(
			conditionFromPreviousObjectSets(objectDeployment.GetGeneration(), prevObjectSets...),
		)

		return
	}

	// Latest objectset revision is also available
	objectDeployment.SetStatusConditions(
		newAvailableCondition(
			metav1.ConditionTrue,
			availableReasonAvailable,
			"Latest Revision is Available.",
			objectDeployment.GetGeneration(),
		),
	)

	controllerOf := make([]corev1alpha1.ControlledObjectReference, 0, len(prevObjectSets)+1)
	for _, os := range prevObjectSets {
		controllerOf = append(controllerOf, getControlledObjRef(os))
	}
	controllerOf = append(controllerOf, getControlledObjRef(currentObjectSet))

	objectDeployment.SetStatusControllerOf(controllerOf)

	updatePausedStatus(currentObjectSet, objectDeployment)
}

func getControlledObjRef(os adapters.ObjectSetAccessor) corev1alpha1.ControlledObjectReference {
	obj := os.ClientObject()
	return corev1alpha1.ControlledObjectReference{
		Kind:      obj.GetObjectKind().GroupVersionKind().Kind,
		Group:     obj.GetObjectKind().GroupVersionKind().Group,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
}

func conditionFromPreviousObjectSets(generation int64, prevObjectSets ...adapters.ObjectSetAccessor) metav1.Condition {
	found, rev := findAvailableRevision(prevObjectSets...)
	if !found {
		return newAvailableCondition(
			metav1.ConditionFalse,
			availableReasonObjectSetUnready,
			"No ObjectSet is available.",
			generation,
		)
	}

	return newAvailableCondition(
		metav1.ConditionTrue,
		availableReasonAvailable,
		fmt.Sprintf("Previous Revision '%s' is still Available.", rev),
		generation,
	)
}

func findAvailableRevision(objectSets ...adapters.ObjectSetAccessor) (bool, string) {
	for _, os := range objectSets {
		availableCond := meta.FindStatusCondition(*os.GetStatusConditions(), corev1alpha1.ObjectSetAvailable)
		if availableCond == nil {
			continue
		}

		var (
			available = availableCond.Status == metav1.ConditionTrue
			currGen   = availableCond.ObservedGeneration == os.ClientObject().GetGeneration()
		)

		if available && currGen {
			return true, os.ClientObject().GetName()
		}
	}

	return false, ""
}

func updatePausedStatus(
	currentObjectSet adapters.ObjectSetAccessor,
	objectDeployment adapters.ObjectDeploymentAccessor,
) {
	pausedCond := meta.FindStatusCondition(*currentObjectSet.GetStatusConditions(), corev1alpha1.ObjectSetPaused)
	if pausedCond != nil && pausedCond.Status == metav1.ConditionTrue {
		objectDeployment.SetStatusConditions(
			newPausedCondition(
				metav1.ConditionTrue,
				pausedReasonPaused,
				"Latest revision is paused: "+pausedCond.Message,
				objectDeployment.GetGeneration(),
			),
		)
	} else {
		objectDeployment.RemoveStatusConditions(corev1alpha1.ObjectDeploymentPaused)
	}
}

func newAvailableCondition(
	status metav1.ConditionStatus, reason availableReason, msg string, generation int64,
) metav1.Condition {
	return metav1.Condition{
		Type:               corev1alpha1.ObjectDeploymentAvailable,
		Status:             status,
		Reason:             reason.String(),
		Message:            msg,
		ObservedGeneration: generation,
	}
}

type availableReason string

func (r availableReason) String() string {
	return string(r)
}

const (
	availableReasonAvailable        availableReason = "Available"
	availableReasonObjectSetUnready availableReason = "ObjectSetUnready"
)

func newProgressingCondition(
	status metav1.ConditionStatus, reason progressingReason, msg string, generation int64,
) metav1.Condition {
	return metav1.Condition{
		Type:               corev1alpha1.ObjectDeploymentProgressing,
		Status:             status,
		Reason:             reason.String(),
		Message:            msg,
		ObservedGeneration: generation,
	}
}

type progressingReason string

func (r progressingReason) String() string {
	return string(r)
}

const (
	progressingReasonIdle                    progressingReason = "Idle"
	progressingReasonLatestRevPendingSuccess progressingReason = "LatestRevisionPendingSuccess"
	progressingReasonProgressing             progressingReason = "Progressing"
)

type pausedReason string

func (r pausedReason) String() string {
	return string(r)
}

const (
	pausedReasonPaused pausedReason = "Paused"
)

func newPausedCondition(
	status metav1.ConditionStatus, reason pausedReason, msg string, generation int64,
) metav1.Condition {
	return metav1.Condition{
		Type:               corev1alpha1.ObjectDeploymentPaused,
		Status:             status,
		Reason:             reason.String(),
		Message:            msg,
		ObservedGeneration: generation,
	}
}
