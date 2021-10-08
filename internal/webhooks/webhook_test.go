package webhooks

import (
	"testing"

	"github.com/stretchr/testify/assert"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/testutil"
)

func Test_validateInstallSpec(t *testing.T) {
	testCases := []struct {
		name             string
		addonInstallSpec addonsv1alpha1.AddonInstallSpec
		expectedErr      error
	}{
		{
			name:             "missing install type",
			addonInstallSpec: addonsv1alpha1.AddonInstallSpec{},
			expectedErr:      errSpecInstallTypeInvalid,
		},
		{
			name: "invalid install type",
			addonInstallSpec: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.AddonInstallType("This is not valid"),
			},
			expectedErr: errSpecInstallTypeInvalid,
		},
		{
			name: "spec.install.ownNamespace required",
			addonInstallSpec: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
			},
			expectedErr: errSpecInstallOwnNamespaceRequired,
		},
		{
			name: "spec.install.allNamespaces required",
			addonInstallSpec: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMAllNamespaces,
			},
			expectedErr: errSpecInstallAllNamespacesRequired,
		},
		{
			name: "spec.install.allNamespaces and *.ownNamespace mutually exclusive",
			addonInstallSpec: addonsv1alpha1.AddonInstallSpec{
				Type:             addonsv1alpha1.OLMAllNamespaces,
				OLMAllNamespaces: &addonsv1alpha1.AddonInstallOLMAllNamespaces{},
				OLMOwnNamespace:  &addonsv1alpha1.AddonInstallOLMOwnNamespace{},
			},
			expectedErr: errSpecInstallConfigMutuallyExclusive,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateInstallSpec(tc.addonInstallSpec)
			assert.EqualValues(t, tc.expectedErr, err)
		})
	}
}

func TestValidateAddonInstallImmutability(t *testing.T) {
	var (
		addonName     = "test-addon"
		catalogSource = "quay.io/osd-addons/reference-addon-index"
	)

	baseAddon := testutil.NewAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
		Type: addonsv1alpha1.OLMAllNamespaces,
		OLMAllNamespaces: &addonsv1alpha1.AddonInstallOLMAllNamespaces{
			AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
				Namespace:          "reference-addon",
				PackageName:        addonName,
				Channel:            "alpha",
				CatalogSourceImage: catalogSource,
			},
		},
	}, addonName)

	testCases := []struct {
		updatedAddon *addonsv1alpha1.Addon
		expectedErr  error
	}{
		{
			updatedAddon: baseAddon,
			expectedErr:  nil,
		},
		{
			updatedAddon: testutil.NewAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMAllNamespaces,
				OLMAllNamespaces: &addonsv1alpha1.AddonInstallOLMAllNamespaces{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          "reference-addon",
						PackageName:        addonName,
						Channel:            "beta", // changed
						CatalogSourceImage: catalogSource,
					},
				},
			}, addonName),
			expectedErr: errInstallImmutable,
		},
		{
			updatedAddon: testutil.NewAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMAllNamespaces,
				OLMAllNamespaces: &addonsv1alpha1.AddonInstallOLMAllNamespaces{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          "reference-addon",
						PackageName:        addonName,
						Channel:            "alpha",
						CatalogSourceImage: "some-other-catalogsource", // changed
					},
				},
			}, addonName),
			expectedErr: nil,
		},
		{
			updatedAddon: testutil.NewAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
			}, addonName),
			expectedErr: errInstallTypeImmutable,
		},
	}

	for _, tc := range testCases {
		err := validateAddonImmutability(tc.updatedAddon, baseAddon)
		assert.EqualValues(t, tc.expectedErr, err)
	}
}
