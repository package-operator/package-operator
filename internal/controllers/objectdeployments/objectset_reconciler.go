package objectdeployments

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type objectSetReconciler struct {
	client                      client.Client
	listObjectSetsForDeployment listObjectSetsForDeploymentFn
	reconcilers                 []objectSetSubReconciler
}

type objectSetSubReconciler interface {
	Reconcile(ctx context.Context,
		currentObjectSet genericObjectSet, prevObjectSets []genericObjectSet, objectDeployment genericObjectDeployment) (ctrl.Result, error)
}

type listObjectSetsForDeploymentFn func(
	ctx context.Context, objectDeployment genericObjectDeployment,
) ([]genericObjectSet, error)

func (o *objectSetReconciler) Reconcile(ctx context.Context, objectDeployment genericObjectDeployment) (ctrl.Result, error) {
	objectSets, err := o.listObjectSetsForDeployment(ctx, objectDeployment)
	currentDeploymentGeneration := fmt.Sprint(objectDeployment.GetGeneration())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("listing objectsets under deployment errored: %w", err)
	}

	var (
		currentObjectSet genericObjectSet
		prevObjectSets   []genericObjectSet
	)

	for _, currObjectSet := range objectSets {
		annotations := currObjectSet.ClientObject().GetAnnotations()
		var (
			templateHashFound       bool
			deploymentRevisionFound bool
			currTemplateHash        string
			deploymentRevision      string
		)
		if annotations != nil {
			currTemplateHash, templateHashFound = annotations[ObjectSetHashAnnotation]
			deploymentRevision, deploymentRevisionFound = annotations[DeploymentRevisionAnnotation]

		}
		if !templateHashFound || !deploymentRevisionFound {
			// The deployment didnt create this objectset, we just ignore?
			continue
		}

		if objectDeployment.GetStatusTemplateHash() == currTemplateHash &&
			deploymentRevision == currentDeploymentGeneration {
			// objectset for this revision already exists
			currentObjectSet = currObjectSet
			continue
		}
		prevObjectSets = append(prevObjectSets, currObjectSet)
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
	objectDeployment genericObjectDeployment,
) {
	var oldRevisionAvailable bool
	if currentObjectSet != nil && currentObjectSet.IsAvailable() {
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

	// Since the latest objectRevision is not present/available, we are progressing to a
	// new revision
	meta.SetStatusCondition(objectDeployment.GetConditions(), metav1.Condition{
		Type:               corev1alpha1.ObjectDeploymentProgressing,
		Status:             metav1.ConditionTrue,
		Reason:             "Progressing",
		Message:            "Progressing to a new ObjectSet.",
		ObservedGeneration: objectDeployment.ClientObject().GetGeneration(),
	})

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
