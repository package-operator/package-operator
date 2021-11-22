package integration_test

import (
	"context"
	"fmt"
	"time"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
)

// This test deploys a version of our addon where InstallPlan and
// CSV never succeed because the deployed operator pod is deliberately
// broken through invalid readiness and liveness probes.
func (s *integrationTestSuite) TestAddon_BrokenSubscription() {

	ctx := context.Background()
	addon := addon_OwnNamespace_TestBrokenSubscription()

	err := integration.Client.Create(ctx, addon)
	s.Require().NoError(err)

	observedCSV := &operatorsv1alpha1.ClusterServiceVersion{
		ObjectMeta: v1.ObjectMeta{
			Namespace: fmt.Sprintf("namespace-%s", uuid),
			Name:      "reference-addon.v0.1.3",
		},
	}

	err = integration.WaitForObject(
		s.T(), 10*time.Minute, observedCSV, "to be Failed",
		func(obj client.Object) (done bool, err error) {
			csv := obj.(*operatorsv1alpha1.ClusterServiceVersion)
			return csv.Status.Phase == operatorsv1alpha1.CSVPhaseFailed, nil
		})
	s.Require().NoError(err)

	{
		observedAddon := &addonsv1alpha1.Addon{}
		err := integration.Client.Get(ctx, client.ObjectKey{
			Name: addon.Name,
		}, observedAddon)
		s.Require().NoError(err)

		s.Assert().Equal(addon.Status.Phase, addonsv1alpha1.PhasePending)
	}

	{
		subscription := &operatorsv1alpha1.Subscription{}
		err := integration.Client.Get(ctx, client.ObjectKey{
			Namespace: addon.Spec.Install.OLMOwnNamespace.Namespace,
			Name:      addon.Name,
		}, subscription)
		s.Require().NoError(err)

		// Force type of `operatorsv1alpha1.SubscriptionStateAtLatest` to `operatorsv1alpha1.SubscriptionState`
		// because it is an untyped string const otherwise.
		var subscriptionAtLatest operatorsv1alpha1.SubscriptionState = operatorsv1alpha1.SubscriptionStateAtLatest
		s.Assert().Equal(subscriptionAtLatest, subscription.Status.State)
		s.Assert().NotEmpty(subscription.Status.Install)
		s.Assert().Equal("reference-addon.v0.1.3", subscription.Status.CurrentCSV)
		s.Assert().Equal("reference-addon.v0.1.3", subscription.Status.InstalledCSV)
	}
	s.T().Cleanup(func() {
		s.addonCleanup(addon, ctx)

		// assert that CatalogSource is gone
		currentCatalogSource := &operatorsv1alpha1.CatalogSource{}
		err = integration.Client.Get(ctx, types.NamespacedName{
			Name:      addon.Name,
			Namespace: addon.Spec.Install.OLMOwnNamespace.Namespace,
		}, currentCatalogSource)
		s.Assert().True(k8sApiErrors.IsNotFound(err), "CatalogSource not deleted: %s", currentCatalogSource.Name)
	})
}
