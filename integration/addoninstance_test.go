package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
)

func TestAddon_AddonInstance(t *testing.T) {
	addonOwnNamespace := &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-firefly",
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: "addon-firefly",
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
				OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          "default",
						CatalogSourceImage: referenceAddonCatalogSourceImageWorking,
						Channel:            "alpha",
						PackageName:        "reference-addon",
					},
				},
			},
		},
	}

	addonAllNamespaces := &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-2425constance",
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: "addon-2425constance",
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{
					Name: "namespace-2425constance",
				},
			},
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMAllNamespaces,
				OLMAllNamespaces: &addonsv1alpha1.AddonInstallOLMAllNamespaces{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          "namespace-2425constance",
						PackageName:        "reference-addon",
						Channel:            "alpha",
						CatalogSourceImage: referenceAddonCatalogSourceImageWorking,
					},
				},
			},
		},
	}

	tests := []struct {
		name            string
		targetNamespace string
		addon           *addonsv1alpha1.Addon
	}{
		{
			name:            "OwnNamespace",
			addon:           addonOwnNamespace,
			targetNamespace: addonOwnNamespace.Spec.Install.OLMOwnNamespace.Namespace,
		},
		{
			name:            "AllNamespaces",
			addon:           addonAllNamespaces,
			targetNamespace: addonAllNamespaces.Spec.Install.OLMAllNamespaces.Namespace,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			addon := test.addon

			err := integration.Client.Create(ctx, addon)
			require.NoError(t, err)
			t.Cleanup(func() {
				err := integration.Client.Delete(ctx, addon)
				if client.IgnoreNotFound(err) != nil {
					t.Logf("could not clean up Addon %s: %v", addon.Name, err)
				}
			})

			err = integration.WaitForObject(
				t, defaultAddonAvailabilityTimeout, addon, "to be Available",
				func(obj client.Object) (done bool, err error) {
					a := obj.(*addonsv1alpha1.Addon)
					return meta.IsStatusConditionTrue(
						a.Status.Conditions, addonsv1alpha1.Available), nil
				})
			require.NoError(t, err)

			// check that there is an addonInstance in the target namespace.
			addonInstance := &addonsv1alpha1.AddonInstance{}
			err = integration.Client.Get(ctx, client.ObjectKey{
				Name:      addonsv1alpha1.DefaultAddonInstanceName,
				Namespace: test.targetNamespace,
			}, addonInstance)
			require.NoError(t, err)
			assert.Equal(t, addonsv1alpha1.DefaultAddonInstanceHeartbeatUpdatePeriod, addonInstance.Spec.HeartbeatUpdatePeriod)
		})
	}
}
