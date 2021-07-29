package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
)

func TestAddon_BrokenSubscription(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	uuid := "c24cd15c-4353-4036-bd86-384046eb4ff8"
	addonName := fmt.Sprintf("addon-%s", uuid)
	addonNamespace := fmt.Sprintf("namespace-%s", uuid)

	addon := &addonsv1alpha1.Addon{
		ObjectMeta: v1.ObjectMeta{
			Name: addonName,
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: addonName,
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{Name: addonNamespace},
			},
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
				OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          addonNamespace,
						CatalogSourceImage: referenceAddonCatalogSourceImageBroken,
						PackageName:        "reference-addon",
						Channel:            "alpha",
					},
				},
			},
		},
	}

	err := integration.Client.Create(ctx, addon)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := integration.Client.Delete(ctx, addon, client.PropagationPolicy("Foreground"))
		if client.IgnoreNotFound(err) != nil {
			t.Logf("could not clean up Addon %s: %v", addon.Name, err)
		}
	})

	observedCSV := &operatorsv1alpha1.ClusterServiceVersion{
		ObjectMeta: v1.ObjectMeta{
			Namespace: fmt.Sprintf("namespace-%s", uuid),
			Name:      "reference-addon.v0.1.3",
		},
	}

	err = integration.WaitForObject(
		t, 10*time.Minute, observedCSV, "to be Failed",
		func(obj client.Object) (done bool, err error) {
			csv := obj.(*operatorsv1alpha1.ClusterServiceVersion)
			return csv.Status.Phase == operatorsv1alpha1.CSVPhaseFailed, nil
		})
	require.NoError(t, err)

	{
		observedAddon := &addonsv1alpha1.Addon{
			ObjectMeta: addon.ObjectMeta,
		}
		err := integration.Client.Get(ctx, client.ObjectKey{
			Name: addon.Name,
		}, observedAddon)
		require.NoError(t, err)

		assert.Equal(t, addon.Status.Phase, addonsv1alpha1.PhasePending)
	}

	{
		subscription := &operatorsv1alpha1.Subscription{}
		err := integration.Client.Get(ctx, client.ObjectKey{
			Namespace: addon.Spec.Install.OLMOwnNamespace.Namespace,
			Name:      addon.Name,
		}, subscription)
		require.NoError(t, err)

		var subscriptionAtLatest operatorsv1alpha1.SubscriptionState = operatorsv1alpha1.SubscriptionStateAtLatest
		assert.Equal(t, subscriptionAtLatest, subscription.Status.State)
		assert.NotEmpty(t, subscription.Status.Install)
		assert.Equal(t, "reference-addon.v0.1.3", subscription.Status.CurrentCSV)
		assert.Equal(t, "reference-addon.v0.1.3", subscription.Status.InstalledCSV)
	}

	// delete Addon
	err = integration.Client.Delete(ctx, addon, client.PropagationPolicy("Foreground"))
	require.NoError(t, err, "delete Addon: %v", addon)

	// wait until Addon is gone
	err = integration.WaitToBeGone(t, addonDeletionTimeout, addon)
	require.NoError(t, err, "wait for Addon to be deleted")

	// assert that CatalogSource is gone
	currentCatalogSource := &operatorsv1alpha1.CatalogSource{}
	err = integration.Client.Get(ctx, types.NamespacedName{
		Name:      addon.Name,
		Namespace: addon.Spec.Install.OLMOwnNamespace.Namespace,
	}, currentCatalogSource)
	assert.True(t, k8sApiErrors.IsNotFound(err), "CatalogSource not deleted: %s", currentCatalogSource.Name)
}
