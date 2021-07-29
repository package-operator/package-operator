package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
)

func TestNamespaceCreation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	addon := &addonsv1alpha1.Addon{
		ObjectMeta: v1.ObjectMeta{
			Name: "addon-c01m94lbi",
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: "addon-c01m94lbi",
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{Name: "namespace-oibabdsoi"},
			},
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
				OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          "namespace-oibabdsoi",
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
	wasAlreadyDeleted := false
	defer func() {
		if !wasAlreadyDeleted {
			err := integration.Client.Delete(ctx, addon)
			if err != nil {
				t.Logf("could not clean up object %s: %v", addon.Name, err)
			}
		}
	}()

	// wait until reconcilation happened
	currentAddon := &addonsv1alpha1.Addon{}
	err = wait.PollImmediate(time.Second, 1*time.Minute, func() (done bool, err error) {
		err = integration.Client.Get(ctx, types.NamespacedName{
			Name: addon.Name,
		}, currentAddon)
		if err != nil {
			t.Logf("error getting Addon: %v", err)
			return false, nil
		}

		isAvailable := meta.IsStatusConditionTrue(currentAddon.Status.Conditions, addonsv1alpha1.Available)
		return isAvailable, nil
	})
	require.NoError(t, err, "wait for Addon to be available: %+v", currentAddon)

	// validate Namespaces
	for _, namespace := range addon.Spec.Namespaces {
		currentNamespace := &corev1.Namespace{}
		err := integration.Client.Get(ctx, types.NamespacedName{
			Name: namespace.Name,
		}, currentNamespace)
		assert.NoError(t, err, "could not get Namespace %s", namespace.Name)

		assert.Equal(t, currentNamespace.Status.Phase, corev1.NamespaceActive)
	}

	// delete Addon
	err = integration.Client.Delete(ctx, addon, client.PropagationPolicy("Foreground"))
	require.NoError(t, err, "delete Addon: %v", addon)

	// wait until Addon is gone
	err = integration.WaitToBeGone(t, addonDeletionTimeout, currentAddon)
	require.NoError(t, err, "wait for Addon to be deleted")

	wasAlreadyDeleted = true

	// assert that all Namespaces are gone
	for _, namespace := range addon.Spec.Namespaces {
		currentNamespace := &corev1.Namespace{}
		err := integration.Client.Get(ctx, types.NamespacedName{
			Name: namespace.Name,
		}, currentNamespace)
		assert.True(t, k8sApiErrors.IsNotFound(err), "Namespace not deleted: %s", namespace.Name)
	}
}
