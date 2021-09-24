package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

func (r *AddonOperatorReconciler) handleAddonOperatorCreation(
	ctx context.Context, log logr.Logger) error {

	defaultAddonOperator := &addonsv1alpha1.AddonOperator{
		ObjectMeta: metav1.ObjectMeta{
			Name: addonsv1alpha1.DefaultAddonOperator,
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
		Reason:             "AddonOperatorReady",
		Message:            "Addon Operator is ready",
		ObservedGeneration: addonOperator.Generation,
	})
	addonOperator.Status.ObservedGeneration = addonOperator.Generation
	addonOperator.Status.Phase = addonsv1alpha1.PhaseReady
	addonOperator.Status.UpdateTimestampNow()
	return r.Status().Update(ctx, addonOperator)
}
