package integration_test

import (
	"context"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
)

func (s *integrationTestSuite) TestAddon() {

	ctx := context.Background()

	addon := addon_OwnNamespace()

	err := integration.Client.Create(ctx, addon)
	s.Require().NoError(err)

	// wait until Addon is available
	err = integration.WaitForObject(
		s.T(), defaultAddonAvailabilityTimeout, addon, "to be Available",
		func(obj client.Object) (done bool, err error) {
			a := obj.(*addonsv1alpha1.Addon)
			return meta.IsStatusConditionTrue(
				a.Status.Conditions, addonsv1alpha1.Available), nil
		})
	s.Require().NoError(err)

	s.Run("test_namespaces", func() {

		for _, namespace := range addon.Spec.Namespaces {
			currentNamespace := &corev1.Namespace{}
			err := integration.Client.Get(ctx, types.NamespacedName{
				Name: namespace.Name,
			}, currentNamespace)
			s.Assert().NoError(err, "could not get Namespace %s", namespace.Name)

			s.Assert().Equal(currentNamespace.Status.Phase, corev1.NamespaceActive)
		}
	})

	s.Run("test_catalogsource", func() {

		currentCatalogSource := &operatorsv1alpha1.CatalogSource{}
		err := integration.Client.Get(ctx, types.NamespacedName{
			Name:      addon.Name,
			Namespace: addon.Spec.Install.OLMOwnNamespace.Namespace,
		}, currentCatalogSource)
		s.Assert().NoError(err, "could not get CatalogSource %s", addon.Name)
		s.Assert().Equal(addon.Spec.Install.OLMOwnNamespace.CatalogSourceImage, currentCatalogSource.Spec.Image)
		s.Assert().Equal(addon.Spec.DisplayName, currentCatalogSource.Spec.DisplayName)
	})

	s.Run("test_subscription_csv", func() {

		subscription := &operatorsv1alpha1.Subscription{}
		{
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
			s.Assert().Equal("reference-addon.v0.1.0", subscription.Status.CurrentCSV)
			s.Assert().Equal("reference-addon.v0.1.0", subscription.Status.InstalledCSV)
		}

		{
			csv := &operatorsv1alpha1.ClusterServiceVersion{}
			err := integration.Client.Get(ctx, client.ObjectKey{
				Namespace: addon.Spec.Install.OLMOwnNamespace.Namespace,
				Name:      subscription.Status.CurrentCSV,
			}, csv)
			s.Require().NoError(err)

			s.Assert().Equal(operatorsv1alpha1.CSVPhaseSucceeded, csv.Status.Phase)
		}
	})

	s.Run("test_subscription_config", func() {

		subscription := &operatorsv1alpha1.Subscription{}

		err := integration.Client.Get(ctx, client.ObjectKey{
			Namespace: addon.Spec.Install.OLMOwnNamespace.Namespace,
			Name:      addon.Name,
		}, subscription)
		s.Require().NoError(err)
		envObjectsPresent := subscription.Spec.Config.Env
		foundEnvMap := make(map[string]string)
		for _, envObj := range envObjectsPresent {
			foundEnvMap[envObj.Name] = envObj.Value
		}
		// assert that the env objects passed while creating the addon are indeed present.
		for _, passedEnvObj := range referenceAddonConfigEnvObjects {
			foundValue, found := foundEnvMap[passedEnvObj.Name]
			s.Assert().True(found, "Passed env variable not found")
			s.Assert().Equal(passedEnvObj.Value, foundValue, "Passed env variable value doesnt match with the one created")
		}
	})

	s.T().Cleanup(func() {

		s.addonCleanup(addon, ctx)

		// assert that CatalogSource is gone
		currentCatalogSource := &operatorsv1alpha1.CatalogSource{}
		err = integration.Client.Get(ctx, types.NamespacedName{
			Name:      addon.Name,
			Namespace: addon.Spec.Install.OLMOwnNamespace.Namespace,
		}, currentCatalogSource)
		s.Assert().True(k8sApiErrors.IsNotFound(err), "CatalogSource not deleted: %s", currentCatalogSource.Name)

		// assert that all Namespaces are gone
		for _, namespace := range addon.Spec.Namespaces {
			currentNamespace := &corev1.Namespace{}
			err := integration.Client.Get(ctx, types.NamespacedName{
				Name: namespace.Name,
			}, currentNamespace)
			s.Assert().True(k8sApiErrors.IsNotFound(err), "Namespace not deleted: %s", namespace.Name)
		}
	})
}
