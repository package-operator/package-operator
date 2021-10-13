package addoninstance

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

// SetCondition sets a certain condition on an AddonInstance corresponding to the provided Addon
// this function will be used by our tenants to report a heartbeat
func SetCondition(ctx context.Context, kubeClient client.Client, condition metav1.Condition, addon addonsv1alpha1.Addon, log logr.Logger) error {
	targetNamespace, err := parseTargetNamespaceFromAddon(addon)
	if err != nil {
		return fmt.Errorf("failed to parse the target namespace from the Addon: %w", err)
	}
	addonInstance := &addonsv1alpha1.AddonInstance{}
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: addonsv1alpha1.DefaultAddonInstanceName, Namespace: targetNamespace}, addonInstance); err != nil {
		return fmt.Errorf("failed to fetch the AddonInstance resource corresponding to the namespace %s: %w", targetNamespace, err)
	}
	if err := upsertAddonInstanceCondition(ctx, kubeClient, addonInstance, condition); err != nil {
		return fmt.Errorf("failed to update the conditions of the AddonInstance resource corresponding to the namespace %s: %w", targetNamespace, err)
	}
	return nil
}

func parseTargetNamespaceFromAddon(addon addonsv1alpha1.Addon) (string, error) {
	var targetNamespace string
	switch addon.Spec.Install.Type {
	case addonsv1alpha1.OLMOwnNamespace:
		if addon.Spec.Install.OLMOwnNamespace == nil ||
			len(addon.Spec.Install.OLMOwnNamespace.Namespace) == 0 {
			// invalid/missing configuration
			return "", fmt.Errorf(".install.spec.olmOwmNamespace.namespace not found")
		}
		targetNamespace = addon.Spec.Install.OLMOwnNamespace.Namespace

	case addonsv1alpha1.OLMAllNamespaces:
		if addon.Spec.Install.OLMAllNamespaces == nil ||
			len(addon.Spec.Install.OLMAllNamespaces.Namespace) == 0 {
			// invalid/missing configuration
			return "", fmt.Errorf(".install.spec.olmAllNamespaces.namespace not found")
		}
		targetNamespace = addon.Spec.Install.OLMAllNamespaces.Namespace
	default:
		// ideally, this should never happen
		// but technically, it is possible to happen if validation webhook is turned off and CRD validation gets bypassed via the `--validate=false` argument
		return "", fmt.Errorf("unsupported install type found: %s. Allowed install types: %+v", addon.Spec.Install.Type, addonsv1alpha1.SupportedInstallTypes)
	}
	return targetNamespace, nil
}

func upsertAddonInstanceCondition(ctx context.Context, kubeClient client.Client, addonInstance *addonsv1alpha1.AddonInstance, condition metav1.Condition) error {
	currentTime := metav1.Now()
	if condition.LastTransitionTime.IsZero() {
		condition.LastTransitionTime = currentTime
	}
	// TODO: confirm that it's not worth tracking the ObservedGeneration at per-condition basis
	meta.SetStatusCondition(&(*addonInstance).Status.Conditions, condition)
	addonInstance.Status.ObservedGeneration = (*addonInstance).Generation
	addonInstance.Status.LastHeartbeatTime = metav1.Now()
	return kubeClient.Status().Update(ctx, addonInstance)
}
