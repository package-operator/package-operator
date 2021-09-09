package integration_test

import (
	"context"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
	"github.com/openshift/addon-operator/internal/testutil"
)

const addonName = "reference-addon-test-install-spec"

func TestAddonInstallSpec(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	testCases := []struct {
		addon *addonsv1alpha1.Addon
		err   *k8sApiErrors.StatusError
	}{
		{
			addon: testutil.NewAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
			}, addonName),
			err: testutil.NewStatusError(".spec.install.ownNamespace is required " +
				"when .spec.install.type = OwnNamespace"),
		},
		{
			addon: testutil.NewAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMAllNamespaces,
			}, addonName),
			err: testutil.NewStatusError(".spec.install.allNamespaces is required " +
				"when .spec.install.type = AllNamespaces"),
		},
		{
			addon: testutil.NewAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
				OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          "reference-addon",
						PackageName:        addonName,
						Channel:            "alpha",
						CatalogSourceImage: referenceAddonCatalogSourceImageWorking,
					},
				},
			}, addonName),
			err: nil,
		},
		{
			addon: testutil.NewAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMAllNamespaces,
				OLMAllNamespaces: &addonsv1alpha1.AddonInstallOLMAllNamespaces{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          "reference-addon",
						PackageName:        addonName,
						Channel:            "alpha",
						CatalogSourceImage: referenceAddonCatalogSourceImageWorking,
					},
				},
			}, addonName),
			err: nil,
		},
	}

	for _, tc := range testCases {
		err := integration.Client.Create(ctx, tc.addon)

		if tc.err != nil {
			if !reflect.DeepEqual(err, tc.err) {
				log.Fatalf("unexpected error: %v\nexpected: %v",
					err, tc.err)
			}
		} else {
			if err != nil {
				log.Fatalf("expected nil error, got: %v", err)
			}

			// clean-up addon
			err := integration.Client.Delete(ctx, tc.addon)
			require.NoError(t, err)

			err = integration.WaitToBeGone(t, 5*time.Minute, tc.addon)
			require.NoError(t, err, "wait for Addon to be deleted")
		}
	}
}

func TestAddonSpecImmutability(t *testing.T) {
	ctx := context.Background()

	addon := testutil.NewAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
		Type: addonsv1alpha1.OLMOwnNamespace,
		OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
			AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
				Namespace:          "reference-addon",
				PackageName:        addonName,
				Channel:            "alpha",
				CatalogSourceImage: referenceAddonCatalogSourceImageWorking,
			},
		},
	}, addonName)

	err := integration.Client.Create(ctx, addon)
	require.NoError(t, err)

	// try to update immutable spec

	resourceVersion := addon.GetResourceVersion()
	addon = testutil.NewAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
		Type: addonsv1alpha1.OLMOwnNamespace,
		OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
			AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
				Namespace:          "reference-addon",
				PackageName:        addonName,
				Channel:            "beta", // changed
				CatalogSourceImage: referenceAddonCatalogSourceImageWorking,
			},
		},
	}, addonName)
	addon.SetResourceVersion(resourceVersion)

	err = integration.Client.Update(ctx, addon)
	expectedErr := testutil.NewStatusError(".spec.install is an immutable field and cannot be updated")

	if !reflect.DeepEqual(err, expectedErr) {
		log.Fatalf("unexpected error: %v\nexpected:%v", err, expectedErr)
	}

	// cleanup
	err = integration.Client.Delete(ctx, addon)
	require.NoError(t, err)

	err = integration.WaitToBeGone(t, 5*time.Minute, addon)
	require.NoError(t, err, "wait for Addon to be deleted")
}
