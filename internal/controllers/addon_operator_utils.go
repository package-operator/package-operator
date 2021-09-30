package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/addon-operator/apis"
	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

// This interface is used for coordinating
// the global pause mutex between AddonReconciler
// and AddopOperatorReconciler
type sharedAddonReconciler interface {
	SetGlobalPause(bool)
	IsPaused() bool
}

func (r *AddonOperatorReconciler) handleAddonOperatorCreation(
	ctx context.Context, log logr.Logger) error {

	defaultAddonOperator := &addonsv1alpha1.AddonOperator{
		ObjectMeta: metav1.ObjectMeta{
			Name: addonsv1alpha1.DefaultAddonOperatorName,
		},
	}

	log.Info("creating default AddonOperator object")
	err := r.Create(ctx, defaultAddonOperator)
	return err
}

// Marks AddonOperator as available
func (r *AddonOperatorReconciler) reportAddonOperatorReadinessStatus(
	ctx context.Context,
	addonOperator *addonsv1alpha1.AddonOperator) error {
	meta.SetStatusCondition(&addonOperator.Status.Conditions, metav1.Condition{
		Type:               addonsv1alpha1.Available,
		Status:             metav1.ConditionTrue,
		Reason:             apis.AddonOperatorReasonReady,
		Message:            "Addon Operator is ready",
		ObservedGeneration: addonOperator.Generation,
	})
	addonOperator.Status.ObservedGeneration = addonOperator.Generation
	addonOperator.Status.Phase = addonsv1alpha1.PhaseReady
	addonOperator.Status.LastHeartbeatTime = metav1.Now()
	return r.Status().Update(ctx, addonOperator)
}

// Marks AddonOperator as paused
func (r *AddonOperatorReconciler) reportAddonOperatorPauseStatus(
	ctx context.Context,
	addonOperator *addonsv1alpha1.AddonOperator) error {
	meta.SetStatusCondition(&addonOperator.Status.Conditions, metav1.Condition{
		Type:               addonsv1alpha1.Paused,
		Status:             metav1.ConditionTrue,
		Reason:             apis.AddonOperatorReasonPaused,
		Message:            "Addon operator is paused",
		ObservedGeneration: addonOperator.Generation,
	})
	addonOperator.Status.ObservedGeneration = addonOperator.Generation
	addonOperator.Status.Phase = addonsv1alpha1.PhaseReady
	addonOperator.Status.LastHeartbeatTime = metav1.Now()
	return r.Status().Update(ctx, addonOperator)
}

// remove Paused condition from AddonOperator
func (r *AddonOperatorReconciler) removeAddonOperatorPauseCondition(
	ctx context.Context, addonOperator *addonsv1alpha1.AddonOperator) error {
	meta.RemoveStatusCondition(&addonOperator.Status.Conditions, addonsv1alpha1.Paused)
	addonOperator.Status.ObservedGeneration = addonOperator.Generation
	addonOperator.Status.Phase = addonsv1alpha1.PhaseReady
	return r.Status().Update(ctx, addonOperator)
}
