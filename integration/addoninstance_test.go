package integration_test

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
)

func (s *integrationTestSuite) TestAddon_AddonInstance() {
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
		s.Run(test.name, func() {
			ctx := context.Background()
			addon := test.addon

			err := integration.Client.Create(ctx, addon)
			s.Require().NoError(err)
			s.T().Cleanup(func() {
				err := integration.Client.Delete(ctx, addon)
				if client.IgnoreNotFound(err) != nil {
					s.T().Logf("could not clean up Addon %s: %v", addon.Name, err)
				}
			})

			err = integration.WaitForObject(
				s.T(), defaultAddonAvailabilityTimeout, addon, "to be Available",
				func(obj client.Object) (done bool, err error) {
					a := obj.(*addonsv1alpha1.Addon)
					return meta.IsStatusConditionTrue(
						a.Status.Conditions, addonsv1alpha1.Available), nil
				})
			s.Require().NoError(err)

			// check that there is an addonInstance in the target namespace.
			addonInstance := &addonsv1alpha1.AddonInstance{}
			err = integration.Client.Get(ctx, client.ObjectKey{
				Name:      addonsv1alpha1.DefaultAddonInstanceName,
				Namespace: test.targetNamespace,
			}, addonInstance)
			s.Require().NoError(err)
			// Default of 10s is hardcoded in AddonInstanceReconciler
			s.Assert().Equal(10*time.Second, addonInstance.Spec.HeartbeatUpdatePeriod.Duration)
		})
	}
}
