package integration_test

import (
	"context"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
)

func (s *integrationTestSuite) TestAddon_OperatorGroup() {
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
		s.Run(test.name, func() {
			ctx := context.Background()
			addon := test.addon

			err := integration.Client.Create(ctx, addon)
			s.Require().NoError(err)
			s.T().Cleanup(func() {
				s.addonCleanup(test.addon, ctx)
			})

			err = integration.WaitForObject(
				s.T(), defaultAddonAvailabilityTimeout, addon, "to be Available",
				func(obj client.Object) (done bool, err error) {
					a := obj.(*addonsv1alpha1.Addon)
					return meta.IsStatusConditionTrue(
						a.Status.Conditions, addonsv1alpha1.Available), nil
				})
			s.Require().NoError(err)

			// check that there is an OperatorGroup in the target namespace.
			operatorGroup := &operatorsv1.OperatorGroup{}
			s.Require().NoError(integration.Client.Get(ctx, client.ObjectKey{
				Name:      addon.Name,
				Namespace: test.targetNamespace,
			}, operatorGroup))
		})
	}
}
