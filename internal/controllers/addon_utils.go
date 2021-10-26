package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

// Handle the deletion of an Addon.
func (r *AddonReconciler) handleAddonDeletion(
	ctx context.Context, addon *addonsv1alpha1.Addon,
) error {
	if err := r.reportTerminationStatus(ctx, addon); err != nil {
		return fmt.Errorf("failed reporting terminiation status: %w", err)
	}

	// Clear from CSV Event Handler
	r.csvEventHandler.Free(addon)

	if controllerutil.ContainsFinalizer(addon, cacheFinalizer) {
		controllerutil.RemoveFinalizer(addon, cacheFinalizer)
		if err := r.Update(ctx, addon); err != nil {
			return fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return nil
}

// Report Addon status to communicate that everything is alright
func (r *AddonReconciler) reportReadinessStatus(
	ctx context.Context, addon *addonsv1alpha1.Addon) error {
	meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
		Type:               addonsv1alpha1.Available,
		Status:             metav1.ConditionTrue,
		Reason:             addonsv1alpha1.AddonReasonFullyReconciled,
		ObservedGeneration: addon.Generation,
	})
	addon.Status.ObservedGeneration = addon.Generation
	addon.Status.Phase = addonsv1alpha1.PhaseReady
	return r.Status().Update(ctx, addon)
}

// Report Addon status to communicate that the Addon is terminating
func (r *AddonReconciler) reportTerminationStatus(
	ctx context.Context, addon *addonsv1alpha1.Addon) error {
	meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
		Type:               addonsv1alpha1.Available,
		Status:             metav1.ConditionFalse,
		Reason:             addonsv1alpha1.AddonReasonTerminating,
		ObservedGeneration: addon.Generation,
	})
	addon.Status.ObservedGeneration = addon.Generation
	addon.Status.Phase = addonsv1alpha1.PhaseTerminating
	return r.Status().Update(ctx, addon)
}

// Report Addon status to communicate that the resource is misconfigured
func (r *AddonReconciler) reportConfigurationError(
	ctx context.Context, addon *addonsv1alpha1.Addon, message string) error {
	addon.Status.ObservedGeneration = addon.Generation
	addon.Status.Phase = addonsv1alpha1.PhaseError
	meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
		Type:    addonsv1alpha1.Available,
		Status:  metav1.ConditionFalse,
		Reason:  addonsv1alpha1.AddonReasonConfigError,
		Message: message,
	})
	addon.Status.ObservedGeneration = addon.Generation
	addon.Status.Phase = addonsv1alpha1.PhaseError
	return r.Status().Update(ctx, addon)
}

// Marks Addon as paused
func (r *AddonReconciler) reportAddonPauseStatus(
	ctx context.Context, reason string,
	addon *addonsv1alpha1.Addon) error {
	meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
		Type:               addonsv1alpha1.Paused,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            "",
		ObservedGeneration: addon.Generation,
	})
	addon.Status.ObservedGeneration = addon.Generation
	addon.Status.Phase = addonsv1alpha1.PhaseReady
	return r.Status().Update(ctx, addon)
}

// remove Paused condition from Addon
func (r *AddonReconciler) removeAddonPauseCondition(ctx context.Context,
	addon *addonsv1alpha1.Addon) error {
	meta.RemoveStatusCondition(&addon.Status.Conditions, addonsv1alpha1.Paused)
	addon.Status.ObservedGeneration = addon.Generation
	addon.Status.Phase = addonsv1alpha1.PhaseReady
	return r.Status().Update(ctx, addon)
}

// Validate addon.Spec.Install then extract
// targetNamespace and catalogSourceImage from it
func (r *AddonReconciler) parseAddonInstallConfig(
	ctx context.Context, log logr.Logger, addon *addonsv1alpha1.Addon) (
	targetNamespace, catalogSourceImage string, stop bool, err error,
) {
	switch addon.Spec.Install.Type {
	case addonsv1alpha1.OLMOwnNamespace:
		if addon.Spec.Install.OLMOwnNamespace == nil ||
			len(addon.Spec.Install.OLMOwnNamespace.Namespace) == 0 {
			// invalid/missing configuration
			return "", "", true, r.reportConfigurationError(ctx, addon,
				".spec.install.ownNamespace.namespace is required when .spec.install.type = OwnNamespace")
		}
		targetNamespace = addon.Spec.Install.OLMOwnNamespace.Namespace
		if len(addon.Spec.Install.OLMOwnNamespace.CatalogSourceImage) == 0 {
			// invalid/missing configuration
			return "", "", true, r.reportConfigurationError(ctx, addon,
				".spec.install.ownNamespacee.catalogSourceImage is required when .spec.install.type = OwnNamespace")
		}
		catalogSourceImage = addon.Spec.Install.OLMOwnNamespace.CatalogSourceImage

	case addonsv1alpha1.OLMAllNamespaces:
		if addon.Spec.Install.OLMAllNamespaces == nil ||
			len(addon.Spec.Install.OLMAllNamespaces.Namespace) == 0 {
			// invalid/missing configuration
			return "", "", true, r.reportConfigurationError(ctx, addon,
				".spec.install.allNamespaces.namespace is required when .spec.install.type = AllNamespaces")
		}
		targetNamespace = addon.Spec.Install.OLMAllNamespaces.Namespace
		if len(addon.Spec.Install.OLMAllNamespaces.CatalogSourceImage) == 0 {
			// invalid/missing configuration
			return "", "", true, r.reportConfigurationError(ctx, addon,
				".spec.install.allNamespaces.catalogSourceImage is required when .spec.install.type = AllNamespaces")
		}
		catalogSourceImage = addon.Spec.Install.OLMAllNamespaces.CatalogSourceImage

	default:
		// Unsupported Install Type
		// This should never happen, unless the schema validation is wrong.
		// The .install.type property is set to only allow known enum values.
		log.Error(fmt.Errorf("invalid Addon install type: %q", addon.Spec.Install.Type), "stopping Addon reconcilation")
		return "", "", true, nil
	}

	return targetNamespace, catalogSourceImage, false, nil
}

// Tests if the controller reference on `wanted` matches the one on `current`
func HasEqualControllerReference(current, wanted metav1.Object) bool {
	currentOwnerRefs := current.GetOwnerReferences()

	var currentControllerRef *metav1.OwnerReference
	for _, ownerRef := range currentOwnerRefs {
		if *ownerRef.Controller {
			currentControllerRef = &ownerRef
			break
		}
	}

	if currentControllerRef == nil {
		return false
	}

	wantedOwnerRefs := wanted.GetOwnerReferences()

	for _, ownerRef := range wantedOwnerRefs {
		// OwnerRef is the same if UIDs match
		if currentControllerRef.UID == ownerRef.UID {
			return true
		}
	}

	return false
}
