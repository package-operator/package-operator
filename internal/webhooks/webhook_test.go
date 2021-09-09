package webhooks

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/testutil"
)

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
			expectedErr: errors.New(".spec.install is an immutable field and cannot be updated"),
		},
	}

	for _, tc := range testCases {
		tc := tc // pin
		err := validateAddonInstallImmutability(*tc.updatedAddon, *baseAddon)
		assert.EqualValues(t, tc.expectedErr, err)
	}
}
