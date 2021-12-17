package addonoperator

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/ocm"
)

// globalPauseManager is an interface used for coordinating
// the global pause mutex between AddonReconciler
// and AddopOperatorReconciler
type globalPauseManager interface {
	EnableGlobalPause(ctx context.Context) error
	DisableGlobalPause(ctx context.Context) error
}

type ocmClientManager interface {
	InjectOCMClient(ctx context.Context, c *ocm.Client) error
}

func (r *AddonOperatorReconciler) handleAddonOperatorCreation(
	ctx context.Context, log logr.Logger) error {

	defaultAddonOperator := &addonsv1alpha1.AddonOperator{
		ObjectMeta: metav1.ObjectMeta{
			Name: addonsv1alpha1.DefaultAddonOperatorName,
		},
	}
	r.Recorder.SetAddonOperatorPaused(false)
	log.Info("creating default AddonOperator object")
	err := r.Create(ctx, defaultAddonOperator)
	return err
}

// Marks AddonOperator as available
func (r *AddonOperatorReconciler) reportAddonOperatorReadinessStatus(
	ctx context.Context,
	addonOperator *addonsv1alpha1.AddonOperator) error {
	meta.SetStatusCondition(&addonOperator.Status.Conditions, metav1.Condition{
		Type:               addonsv1alpha1.AddonOperatorAvailable,
		Status:             metav1.ConditionTrue,
		Reason:             addonsv1alpha1.AddonOperatorReasonReady,
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
		Type:               addonsv1alpha1.AddonOperatorPaused,
		Status:             metav1.ConditionTrue,
		Reason:             addonsv1alpha1.AddonOperatorReasonPaused,
		Message:            "Addon operator is paused",
		ObservedGeneration: addonOperator.Generation,
	})
	addonOperator.Status.ObservedGeneration = addonOperator.Generation
	addonOperator.Status.LastHeartbeatTime = metav1.Now()
	return r.Status().Update(ctx, addonOperator)
}

// remove Paused condition from AddonOperator
func (r *AddonOperatorReconciler) removeAddonOperatorPauseCondition(
	ctx context.Context, addonOperator *addonsv1alpha1.AddonOperator) error {
	meta.RemoveStatusCondition(&addonOperator.Status.Conditions, addonsv1alpha1.Paused)
	addonOperator.Status.ObservedGeneration = addonOperator.Generation
	return r.Status().Update(ctx, addonOperator)
}
