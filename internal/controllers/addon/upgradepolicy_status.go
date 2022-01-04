package addon

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/ocm"
)

// reconciler-facing wrapper around ocm.PatchUpgradePolicy that makes it
// easier to record OCM API metrics, and unit test the instrumentation.
// This also allows us to re-use the Recorder in AddonReconciler for recording
// OCM API metrics, rather than passing it down to the ocmClient object.
func (r *AddonReconciler) handlePatchUpgradePolicy(ctx context.Context,
	req ocm.UpgradePolicyPatchRequest) error {
	if r.Recorder != nil {
		// TODO: do not count metrics when API returns 5XX response
		timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
			us := v * 1000000 // convert to microseconds
			r.Recorder.RecordOCMAPIRequests(us)
		}))
		defer timer.ObserveDuration()
	}
	_, err := r.ocmClient.PatchUpgradePolicy(ctx, req)
	return err
}

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
		err := r.handlePatchUpgradePolicy(ctx, ocm.UpgradePolicyPatchRequest{
			ID:          addon.Spec.UpgradePolicy.ID,
			Value:       ocm.UpgradePolicyValueStarted,
			Description: "Upgrading addon.",
		})
		if err != nil {
			return fmt.Errorf("patching UpgradePolicy endpoint: %w", err)
		}

		addon.Status.UpgradePolicy = &addonsv1alpha1.AddonUpgradePolicyStatus{
			ID:                 addon.Spec.UpgradePolicy.ID,
			Value:              addonsv1alpha1.AddonUpgradePolicyValueStarted,
			ObservedGeneration: addon.Generation,
		}
		return nil
	}

	if !meta.IsStatusConditionTrue(addon.Status.Conditions, addonsv1alpha1.Available) {
		// Addon is not healthy or not done with the upgrade.
		return nil
	}

	// Addon is healthy and we have not yet reported the upgrade as completed,
	// let's do that :)
	err := r.handlePatchUpgradePolicy(ctx, ocm.UpgradePolicyPatchRequest{
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
	return nil
}
