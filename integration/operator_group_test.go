package integration_test

import (
	"context"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
)

func (s *integrationTestSuite) TestAddon_OperatorGroup() {
	addonOwnNamespace := addon_OwnNamespace()
	addonAllNamespaces := addon_AllNamespaces()

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

			s.T().Cleanup(func() {
				s.addonCleanup(test.addon, ctx)
			})
		})
	}
}
