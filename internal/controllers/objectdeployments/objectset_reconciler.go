package objectdeployments

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type objectSetReconciler struct {
	client                      client.Client
	listObjectSetsForDeployment listObjectSetsForDeploymentFn
	reconcilers                 []objectSetSubReconciler
}

type objectSetSubReconciler interface {
	Reconcile(ctx context.Context,
		currentObjectSet genericObjectSet, prevObjectSets []genericObjectSet, objectDeployment objectDeploymentAccessor) (ctrl.Result, error)
}

type listObjectSetsForDeploymentFn func(
	ctx context.Context, objectDeployment objectDeploymentAccessor,
) ([]genericObjectSet, error)

func (o *objectSetReconciler) Reconcile(ctx context.Context, objectDeployment objectDeploymentAccessor) (ctrl.Result, error) {
	objectSets, err := o.listObjectSetsForDeployment(ctx, objectDeployment)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("listing objectsets under deployment errored: %w", err)
	}

	// Delay any action until all ObjectSets under management report .status.revision
	for _, objectSet := range objectSets {
		if objectSet.GetRevision() == 0 {
			return ctrl.Result{}, nil
		}
	}

	// objectSets is already sorted ascending by .status.revision
	// check if the latest revision is up-to-date, by comparing their hash.
	var (
		currentObjectSet genericObjectSet
		prevObjectSets   []genericObjectSet
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

func (o *objectSetReconciler) setObjectDeploymentStatus(ctx context.Context,
	currentObjectSet genericObjectSet,
	prevObjectSets []genericObjectSet,
	objectDeployment objectDeploymentAccessor,
) {
	var (
		oldRevisionAvailable      bool
		currentObjectSetSucceeded bool
	)
	if currentObjectSet != nil {
		// map conditions
		// -> copy mapped status conditions
		for _, condition := range currentObjectSet.GetConditions() {
			if condition.ObservedGeneration !=
				currentObjectSet.ClientObject().GetGeneration() {
				// mapped condition is outdated
				continue
			}

			if !strings.Contains(condition.Type, "/") {
				// mapped conditions are prefixed
				continue
			}

			meta.SetStatusCondition(objectDeployment.GetConditions(), metav1.Condition{
				Type:               condition.Type,
				Status:             condition.Status,
				Reason:             condition.Reason,
				Message:            condition.Message,
				ObservedGeneration: objectDeployment.ClientObject().GetGeneration(),
			})
		}

		if currentObjectSet.IsAvailable() {
			// Latest revision is available, so we are no longer progressing.
			meta.SetStatusCondition(objectDeployment.GetConditions(), metav1.Condition{
				Type:               corev1alpha1.ObjectDeploymentProgressing,
				Status:             metav1.ConditionFalse,
				Reason:             "Idle",
				Message:            "Update concluded.",
				ObservedGeneration: objectDeployment.ClientObject().GetGeneration(),
			})
			// Latest objectset revision is also available
			meta.SetStatusCondition(objectDeployment.GetConditions(), metav1.Condition{
				Type:               corev1alpha1.ObjectDeploymentAvailable,
				Status:             metav1.ConditionTrue,
				Reason:             "Available",
				Message:            "Latest ObjectSet is Available.",
				ObservedGeneration: objectDeployment.ClientObject().GetGeneration(),
			})
			return
		}

		succeededCond := meta.FindStatusCondition(currentObjectSet.GetConditions(), corev1alpha1.ObjectSetSucceeded)
		currentObjectSetSucceeded = succeededCond != nil
	}

	// latest object revision is not present or available
	for _, objectSet := range prevObjectSets {
		availableCond := meta.FindStatusCondition(
			objectSet.GetConditions(),
			corev1alpha1.ObjectSetAvailable,
		)
		if availableCond != nil &&
			availableCond.Status == metav1.ConditionTrue &&
			availableCond.ObservedGeneration ==
				objectSet.ClientObject().GetGeneration() {
			oldRevisionAvailable = true
			break
		}
	}

	if !currentObjectSetSucceeded {
		// Latest revision did not yet succeed -> we are still progressing.
		meta.SetStatusCondition(objectDeployment.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectDeploymentProgressing,
			Status:             metav1.ConditionTrue,
			Reason:             "Progressing",
			Message:            "Progressing to a new ObjectSet.",
			ObservedGeneration: objectDeployment.ClientObject().GetGeneration(),
		})
	}

	// Atleast one objectset old revision is still ava
	if oldRevisionAvailable {
		meta.SetStatusCondition(objectDeployment.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectDeploymentAvailable,
			Status:             metav1.ConditionTrue,
			Reason:             "Available",
			Message:            "At least one revision ObjectSet is Available.",
			ObservedGeneration: objectDeployment.ClientObject().GetGeneration(),
		})
	} else {
		meta.SetStatusCondition(objectDeployment.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetAvailable,
			Status:             metav1.ConditionFalse,
			Reason:             "ObjectSetUnready",
			Message:            "No ObjectSet is available.",
			ObservedGeneration: objectDeployment.ClientObject().GetGeneration(),
		})
	}
}
