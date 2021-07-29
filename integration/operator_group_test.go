package integration_test

import (
	"context"
	"testing"
	"time"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
)

func TestAddon_OperatorGroup(t *testing.T) {
	addonOwnNamespace := &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-fuccniy3l4",
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: "addon-fuccniy3l4",
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
			Name: "addon-7dfn114yv1",
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: "addon-7dfn114yv1",
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{
					Name: "namespace-7dfn114yv1",
				},
			},
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMAllNamespaces,
				OLMAllNamespaces: &addonsv1alpha1.AddonInstallOLMAllNamespaces{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          "namespace-7dfn114yv1",
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
				t, 1*time.Minute, addon, "to be Available",
				func(obj client.Object) (done bool, err error) {
					a := obj.(*addonsv1alpha1.Addon)
					return meta.IsStatusConditionTrue(
						a.Status.Conditions, addonsv1alpha1.Available), nil
				})
			require.NoError(t, err)

			// check that there is an OperatorGroup in the target namespace.
			operatorGroup := &operatorsv1.OperatorGroup{}
			require.NoError(t, integration.Client.Get(ctx, client.ObjectKey{
				Name:      addon.Name,
				Namespace: test.targetNamespace,
			}, operatorGroup))
		})
	}
}
