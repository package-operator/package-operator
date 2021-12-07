package integration_test

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
	"github.com/openshift/addon-operator/internal/ocm"
	"github.com/openshift/addon-operator/internal/testutil"
)

func (s *integrationTestSuite) TestUpgradePolicyReporting() {
	if !testutil.IsApiMockEnabled() {
		s.T().Skip("skipping OCM tests since api mock execution is disabled")
	}

	ctx := context.Background()
	addon := addon_OwnNamespace_UpgradePolicyReporting()

	err := integration.Client.Create(ctx, addon)
	s.Require().NoError(err)

	s.T().Cleanup(func() {
		s.addonCleanup(addon, ctx)
	})

	// wait until Addon is available
	err = integration.WaitForObject(
		s.T(), defaultAddonAvailabilityTimeout, addon, "to be Available",
		func(obj client.Object) (done bool, err error) {
			a := obj.(*addonsv1alpha1.Addon)
			return meta.IsStatusConditionTrue(
				a.Status.Conditions, addonsv1alpha1.Available), nil
		})
	s.Require().NoError(err)

	s.Run("reports to upgrade policy endpoint", func() {
		res, err := integration.OCMClient.GetUpgradePolicy(ctx, ocm.UpgradePolicyGetRequest{ID: addon.Spec.UpgradePolicy.ID})
		s.Require().NoError(err)

		s.Assert().Equal(ocm.UpgradePolicyValueCompleted, res.Value)
	})
}
