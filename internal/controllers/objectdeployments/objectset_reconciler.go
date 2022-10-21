package objectdeployments

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"

	"github.com/getsentry/sentry-go"
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
	objectDeployment genericObjectDeployment,
) (res ctrl.Result) {
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
		// ensure to clear ProgressDeadlineExceeded condition, if present
		meta.RemoveStatusCondition(objectDeployment.GetConditions(), corev1alpha1.ObjectDeploymentProgressDeadlineExceeded)
		return res
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

	// Since the latest objectRevision is not present/available,
	// we are progressing to a new revision
	meta.SetStatusCondition(objectDeployment.GetConditions(), metav1.Condition{
		Type:               corev1alpha1.ObjectDeploymentProgressing,
		Status:             metav1.ConditionTrue,
		Reason:             "Progressing",
		Message:            "Progressing to a new ObjectSet.",
		ObservedGeneration: objectDeployment.ClientObject().GetGeneration(),
	})

	progressDeadlineSeconds := objectDeployment.GetProgressDeadlineSeconds()
	if progressDeadlineSeconds != nil {
		progressingCondition := meta.FindStatusCondition(*objectDeployment.GetConditions(), corev1alpha1.ObjectDeploymentProgressing)
		// We should resync this deployment at some point in the future[1]
		// and check whether it has timed out.
		//
		// For example, if a Deployment updated its Progressing condition 3 minutes ago and has a
		// deadline of 10 minutes, it would need to be resynced for a progress check after 7 minutes.
		progressDeadlineDuration := time.Duration(*progressDeadlineSeconds) * time.Second
		progressDeadline := progressingCondition.LastTransitionTime.Add(progressDeadlineDuration)
		res.RequeueAfter = time.Until(progressDeadline)

		if progressDeadline.After(time.Now()) {
			// Deadline exceeded
			meta.SetStatusCondition(objectDeployment.GetConditions(), metav1.Condition{
				Type:    corev1alpha1.ObjectDeploymentProgressDeadlineExceeded,
				Status:  metav1.ConditionTrue,
				Reason:  "ProgressDeadlineExceeded",
				Message: fmt.Sprintf("ObjectDeployment exceeded it's progress deadline of %s", progressDeadlineDuration),
			})
			//nolint:goerr113
			sentry.
				CaptureException(fmt.Errorf(
					"ObjectDeployment %q exceeded it's progress deadline of %s",
					client.ObjectKeyFromObject(objectDeployment.ClientObject()),
					progressDeadlineDuration),
				)
		}
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
	return res
}
