package e2e_test

import (
	"context"
	"testing"
	"time"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
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
	"github.com/openshift/addon-operator/e2e"
)

func TestAddon_CatalogSource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	addon := &addonsv1alpha1.Addon{
		ObjectMeta: v1.ObjectMeta{
			Name:      "addon-oisafbo12",
			Namespace: "default",
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: "addon-oisafbo12",
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{Name: "namespace-onbgdions"},
				{Name: "namespace-pioghfndb"},
			},
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OwnNamespace,
				OwnNamespace: &addonsv1alpha1.AddonInstallOwnNamespace{
					AddonInstallCommon: addonsv1alpha1.AddonInstallCommon{
						Namespace:          "namespace-onbgdions",
						CatalogSourceImage: testCatalogSourceImage,
					},
				},
			},
		},
	}

	err := e2e.Client.Create(ctx, addon)
	require.NoError(t, err)

	// clean up addon resource in case it
	// was leaked because of a failed test
	wasAlreadyDeleted := false
	defer func() {
		if !wasAlreadyDeleted {
			err := e2e.Client.Delete(ctx, addon)
			if err != nil {
				t.Logf("could not clean up object %s: %v", addon.Name, err)
			}
		}
	}()

	// wait until reconciliation happened
	currentAddon := &addonsv1alpha1.Addon{}
	err = wait.PollImmediate(time.Second, 1*time.Minute, func() (done bool, err error) {
		err = e2e.Client.Get(ctx, types.NamespacedName{
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

	// validate CatalogSource
	{
		currentCatalogSource := &operatorsv1alpha1.CatalogSource{}
		err := e2e.Client.Get(ctx, types.NamespacedName{
			Name:      addon.Name,
			Namespace: addon.Spec.Install.OwnNamespace.Namespace,
		}, currentCatalogSource)
		assert.NoError(t, err, "could not get CatalogSource %s", addon.Name)
		assert.Equal(t, addon.Spec.Install.OwnNamespace.CatalogSourceImage, currentCatalogSource.Spec.Image)
		assert.Equal(t, addon.Spec.DisplayName, currentCatalogSource.Spec.DisplayName)
	}

	// delete Addon
	err = e2e.Client.Delete(ctx, addon, client.PropagationPolicy("Foreground"))
	require.NoError(t, err, "delete Addon: %v", addon)

	// wait until Addon is gone
	err = e2e.WaitToBeGone(t, 30*time.Second, currentAddon)
	require.NoError(t, err, "wait for Addon to be deleted")

	wasAlreadyDeleted = true

	// assert that all Namespaces are gone
	for _, namespace := range addon.Spec.Namespaces {
		currentNamespace := &corev1.Namespace{}
		err := e2e.Client.Get(ctx, types.NamespacedName{
			Name: namespace.Name,
		}, currentNamespace)
		assert.True(t, k8sApiErrors.IsNotFound(err), "Namespace not deleted: %s", namespace.Name)
	}
}
