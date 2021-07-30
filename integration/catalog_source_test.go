package integration_test

import (
	"context"
	"testing"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
)

func TestAddon_CatalogSource(t *testing.T) {
	t.Parallel()

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
	require.NoError(t, err)

	// clean up addon resource in case it
	// was leaked because of a failed test
	t.Cleanup(func() {
		err := integration.Client.Delete(ctx, addon, client.PropagationPolicy("Foreground"))
		if client.IgnoreNotFound(err) != nil {
			t.Logf("could not clean up Addon %s: %v", addon.Name, err)
		}
	})

	// wait until Addon is available
	err = integration.WaitForObject(
		t, defaultAddonAvailabilityTimeout, addon, "to be Available",
		func(obj client.Object) (done bool, err error) {
			a := obj.(*addonsv1alpha1.Addon)
			return meta.IsStatusConditionTrue(
				a.Status.Conditions, addonsv1alpha1.Available), nil
		})
	require.NoError(t, err)

	// validate CatalogSource
	{
		currentCatalogSource := &operatorsv1alpha1.CatalogSource{}
		err := integration.Client.Get(ctx, types.NamespacedName{
			Name:      addon.Name,
			Namespace: addon.Spec.Install.OLMOwnNamespace.Namespace,
		}, currentCatalogSource)
		assert.NoError(t, err, "could not get CatalogSource %s", addon.Name)
		assert.Equal(t, addon.Spec.Install.OLMOwnNamespace.CatalogSourceImage, currentCatalogSource.Spec.Image)
		assert.Equal(t, addon.Spec.DisplayName, currentCatalogSource.Spec.DisplayName)
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
