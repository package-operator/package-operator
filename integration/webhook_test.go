package integration_test

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"testing"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
	"github.com/stretchr/testify/require"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	CATALOG_SOURCE_URL = "quay.io/osd-addons/reference-addon-index@sha256:58cb1c4478a150dc44e6c179d709726516d84db46e4e130a5227d8b76456b5bd"
	ADDON_NAME         = "reference-addon"
)

func TestAddonInstallSpec(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		addon *addonsv1alpha1.Addon
		err   *k8sApiErrors.StatusError
	}{
		{
			addon: newAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
			}),
			err: newStatusError(".spec.install.ownNamespace is required " +
				"when .spec.install.type = OwnNamespace"),
		},
		{
			addon: newAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMAllNamespaces,
			}),
			err: newStatusError(".spec.install.allNamespaces is required " +
				"when .spec.install.type = AllNamespaces"),
		},
		{
			addon: newAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
				OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          "reference-addon",
						PackageName:        ADDON_NAME,
						Channel:            "alpha",
						CatalogSourceImage: CATALOG_SOURCE_URL,
					},
				},
			}),
			err: nil,
		},
		{
			addon: newAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMAllNamespaces,
				OLMAllNamespaces: &addonsv1alpha1.AddonInstallOLMAllNamespaces{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          "reference-addon",
						PackageName:        ADDON_NAME,
						Channel:            "alpha",
						CatalogSourceImage: CATALOG_SOURCE_URL,
					},
				},
			}),
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
			if err := integration.Client.Delete(ctx, tc.addon); err != nil {
				log.Fatalf("failed to delete addon object: %v", err)
			}
		}
	}
}

func TestAddonSpecImmutability(t *testing.T) {
	ctx := context.Background()

	addon := newAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
		Type: addonsv1alpha1.OLMOwnNamespace,
		OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
			AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
				Namespace:          "reference-addon",
				PackageName:        ADDON_NAME,
				Channel:            "alpha",
				CatalogSourceImage: CATALOG_SOURCE_URL,
			},
		},
	})

	err := integration.Client.Create(ctx, addon)
	require.NoError(t, err)

	// try to update immutable spec

	resourceVersion := addon.GetResourceVersion()
	addon = newAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
		Type: addonsv1alpha1.OLMOwnNamespace,
		OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
			AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
				Namespace:          "reference-addon",
				PackageName:        ADDON_NAME,
				Channel:            "beta", // changed
				CatalogSourceImage: CATALOG_SOURCE_URL,
			},
		},
	})
	addon.SetResourceVersion(resourceVersion)

	err = integration.Client.Update(ctx, addon)
	expectedErr := newStatusError(".spec.install is an immutable field and cannot be updated")

	if !reflect.DeepEqual(err, expectedErr) {
		log.Fatalf("unexpected error: %v\nexpected:%v", err, expectedErr)
	}

	// cleanup
	err = integration.Client.Delete(ctx, addon)
	require.NoError(t, err)
}

func newStatusError(msg string) *k8sApiErrors.StatusError {
	return &k8sApiErrors.StatusError{
		ErrStatus: v1.Status{
			Status: "Failure",
			Message: fmt.Sprintf("%s %s",
				"admission webhook \"vaddons.managed.openshift.io\" denied the request:",
				msg),
			Reason: v1.StatusReason(msg),
			Code:   403,
		},
	}
}

func newAddonWithInstallSpec(installSpec addonsv1alpha1.AddonInstallSpec) *addonsv1alpha1.Addon {
	return &addonsv1alpha1.Addon{
		ObjectMeta: v1.ObjectMeta{
			Name: ADDON_NAME,
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: "An example addon",
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{Name: "reference-addon"},
			},
			Install: installSpec,
		},
	}
}
