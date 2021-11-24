package addon

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/ocm"
)

func (r *AddonReconciler) handleUpgradePolicyStatusReporting(
	ctx context.Context,
	log logr.Logger,
	addon *addonsv1alpha1.Addon,
) error {
	if addon.Spec.UpgradePolicy == nil {
		// Addons without UpgradePolicy can be skipped silently.
		return nil
	}

	if addon.Status.UpgradePolicy != nil &&
		addon.Status.UpgradePolicy.ID == addon.Spec.UpgradePolicy.ID &&
		addon.Status.UpgradePolicy.Value == addonsv1alpha1.AddonUpgradePolicyValueCompleted {
		// Addon upgrade status was already reported and is in a final transition state.
		// Nothing to do, till the next upgrade is issued.
		return nil
	}

	r.ocmClientMux.RLock()
	defer r.ocmClientMux.RUnlock()

	if r.ocmClient == nil {
		// OCM Client is not initialized.
		// Either the AddonOperatorReconciler did not yet create and inject the client or
		// the AddonOperator CR is not configured for OCM status reporting.
		//
		// All Addons will be requeued when the client becomes available for the first time.
		log.Info("delaying Addon status reporting to UpgradePolicy endpoint until OCM client is initialized")
		return nil
	}

	if addon.Status.UpgradePolicy == nil ||
		addon.Status.UpgradePolicy.ID != addon.Spec.UpgradePolicy.ID {
		// The current upgrade policy never received a status update.
		// Tell them: "we are working on it"
		_, err := r.ocmClient.PatchUpgradePolicy(ctx, ocm.UpgradePolicyPatchRequest{
			ID:          addon.Spec.UpgradePolicy.ID,
			Value:       ocm.UpgradePolicyValueStarted,
			Description: "Upgrading addon.",
		})
		if err != nil {
			return fmt.Errorf("patching UpgradePolicy endpoint: %w", err)
		}

		log.Info("updating Addon status!")
		addon.Status.UpgradePolicy = &addonsv1alpha1.AddonUpgradePolicyStatus{
			ID:                 addon.Spec.UpgradePolicy.ID,
			Value:              addonsv1alpha1.AddonUpgradePolicyValueStarted,
			ObservedGeneration: addon.Generation,
		}
		if err := r.Status().Update(ctx, addon); err != nil {
			return fmt.Errorf("updating Addon status: %w", err)
		}
		log.Info("updated Addon status!")
		return nil
	}

	if !meta.IsStatusConditionTrue(addon.Status.Conditions, addonsv1alpha1.Available) {
		// Addon is not healthy or not done with the upgrade.
		return nil
	}

	// Addon is healthy and we have not yet reported the upgrade as completed,
	// let's do that :)

	_, err := r.ocmClient.PatchUpgradePolicy(ctx, ocm.UpgradePolicyPatchRequest{
		ID:          addon.Spec.UpgradePolicy.ID,
		Value:       ocm.UpgradePolicyValueCompleted,
		Description: "Addon was healthy at least once.",
	})
	if err != nil {
		return fmt.Errorf("patching UpgradePolicy endpoint: %w", err)
	}

	addon.Status.UpgradePolicy = &addonsv1alpha1.AddonUpgradePolicyStatus{
		ID:                 addon.Spec.UpgradePolicy.ID,
		Value:              addonsv1alpha1.AddonUpgradePolicyValueCompleted,
		ObservedGeneration: addon.Generation,
	}
	if err := r.Status().Update(ctx, addon); err != nil {
		return fmt.Errorf("updating Addon status: %w", err)
	}
	return nil
}
