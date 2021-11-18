package integration_test

import (
	"context"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
)

func (s *integrationTestSuite) TestAddon() {
	s.T().Parallel()

	ctx := context.Background()

	addon := &addonsv1alpha1.Addon{
		ObjectMeta: v1.ObjectMeta{
			Name: "addon-oisafbo12",
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: "addon-oisafbo12",
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{Name: "namespace-onbgdions"},
				{Name: "namespace-pioghfndb"},
			},
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
				OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          "namespace-onbgdions",
						CatalogSourceImage: referenceAddonCatalogSourceImageWorking,
						Channel:            "alpha",
						PackageName:        "reference-addon",
					},
				},
			},
		},
	}

	err := integration.Client.Create(ctx, addon)
	s.Require().NoError(err)

	// clean up addon resource in case it
	// was leaked because of a failed test
	s.T().Cleanup(func() {
		err := integration.Client.Delete(ctx, addon, client.PropagationPolicy("Foreground"))
		if client.IgnoreNotFound(err) != nil {
			s.T().Logf("could not clean up Addon %s: %v", addon.Name, err)
		}
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

	s.Run("test_namespaces", func() {
		s.T().Parallel()

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
		s.T().Parallel()

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
		s.T().Parallel()

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

	s.T().Cleanup(func() {
		// delete Addon
		err = integration.Client.Delete(ctx, addon, client.PropagationPolicy("Foreground"))
		s.Require().NoError(err, "delete Addon: %v", addon)

		// wait until Addon is gone
		err = integration.WaitToBeGone(s.T(), defaultAddonDeletionTimeout, addon)
		s.Require().NoError(err, "wait for Addon to be deleted")

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
