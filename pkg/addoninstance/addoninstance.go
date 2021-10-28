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
func SetCondition(ctx context.Context, cacheBackedKubeClient client.Client, condition metav1.Condition, addonName string, log logr.Logger) error {
	addon := &addonsv1alpha1.Addon{}
	if err := cacheBackedKubeClient.Get(ctx, types.NamespacedName{Name: addonName}, addon); err != nil {
		return err
	}
	targetNamespace, err := parseTargetNamespaceFromAddon(*addon)
	if err != nil {
		return fmt.Errorf("failed to parse the target namespace from the Addon: %w", err)
	}
	addonInstance := &addonsv1alpha1.AddonInstance{}
	if err := cacheBackedKubeClient.Get(ctx, types.NamespacedName{Name: addonsv1alpha1.DefaultAddonInstanceName, Namespace: targetNamespace}, addonInstance); err != nil {
		return fmt.Errorf("failed to fetch the AddonInstance resource corresponding to the namespace %s: %w", targetNamespace, err)
	}
	if err := upsertAddonInstanceCondition(ctx, cacheBackedKubeClient, addonInstance, condition); err != nil {
		return fmt.Errorf("failed to update the conditions of the AddonInstance resource corresponding to the namespace %s: %w", targetNamespace, err)
	}
	return nil
}

func upsertAddonInstanceCondition(ctx context.Context, cacheBackedKubeClient client.Client, addonInstance *addonsv1alpha1.AddonInstance, condition metav1.Condition) error {
	currentTime := metav1.Now()
	fmt.Printf("\nWriting condition: %+v", condition)
	if condition.LastTransitionTime.IsZero() {
		condition.LastTransitionTime = currentTime
	}
	// TODO: confirm that it's not worth tracking the ObservedGeneration at per-condition basis
	meta.SetStatusCondition(&(*addonInstance).Status.Conditions, condition)
	addonInstance.Status.ObservedGeneration = (*addonInstance).Generation
	addonInstance.Status.LastHeartbeatTime = metav1.Now()
	return cacheBackedKubeClient.Status().Update(ctx, addonInstance)
}
