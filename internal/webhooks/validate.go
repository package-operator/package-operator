package webhooks

import (
	"errors"
	"fmt"
	"reflect"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

func validateInstallSpec(addon addonsv1alpha1.Addon) error {
	switch addon.Spec.Install.Type {
	case addonsv1alpha1.OLMOwnNamespace:
		if addon.Spec.Install.OLMOwnNamespace == nil {
			// missing configuration
			return errors.New(".spec.install.ownNamespace is required when .spec.install.type = OwnNamespace")
		}

		// The two conditions below are usually never evaluated at the webhook
		// because schema validation should handle it for us
		if len(addon.Spec.Install.OLMOwnNamespace.Namespace) == 0 {
			// invalid/missing configuration
			return errors.New(".spec.install.ownNamespace.namespace is required when .spec.install.type = OwnNamespace")
		}

		if len(addon.Spec.Install.OLMOwnNamespace.CatalogSourceImage) == 0 {
			// invalid/missing configuration
			return errors.New(".spec.install.ownNamespacee.catalogSourceImage is required when .spec.install.type = OwnNamespace")
		}
		return nil

	case addonsv1alpha1.OLMAllNamespaces:
		if addon.Spec.Install.OLMAllNamespaces == nil {
			// missing configuration
			return errors.New(".spec.install.allNamespaces is required when .spec.install.type = AllNamespaces")
		}

		// The two conditions below are usually never evaluated at the webhook
		// because schema validation should handle it for us
		if len(addon.Spec.Install.OLMAllNamespaces.Namespace) == 0 {
			// invalid/missing configuration
			return errors.New(".spec.install.allNamespaces.namespace is required when .spec.install.type = AllNamespaces")
		}

		if len(addon.Spec.Install.OLMAllNamespaces.CatalogSourceImage) == 0 {
			// invalid/missing configuration
			return errors.New(".spec.install.allNamespaces.catalogSourceImage is required when .spec.install.type = AllNamespaces")
		}
		return nil

	default:
		// Unsupported Install Type
		// This should never happen, unless the schema validation is wrong.
		// The .install.type property is set to only allow known enum values.
		return fmt.Errorf("invalid Addon install type: %q", addon.Spec.Install.Type)
	}
}

func validateAddonInstallImmutability(addon, oldAddon addonsv1alpha1.Addon) error {
	if !reflect.DeepEqual(addon.Spec.Install, oldAddon.Spec.Install) {
		return errors.New(".spec.install is an immutable field and cannot be updated")
	}
	return nil
}
