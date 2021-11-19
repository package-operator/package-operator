package integration_test

import (
	"context"
	"fmt"
	"reflect"
	"time"

	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
	"github.com/openshift/addon-operator/internal/testutil"
)

func (s *integrationTestSuite) TestAddonInstallSpec() {
	if !testutil.IsWebhookServerEnabled() {
		s.T().Skip("skipping test as webhook server execution is disabled")
	}

	ctx := context.Background()
	addonName := "reference-addon-test-install-spec"

	testCases := []struct {
		addon *addonsv1alpha1.Addon
		err   *k8sApiErrors.StatusError
	}{
		{
			addon: testutil.NewAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
			}, addonName),
			err: testutil.NewStatusError(".spec.install.olmOwnNamespace is required when .spec.install.type = OLMOwnNamespace"),
		},
		{
			addon: testutil.NewAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMAllNamespaces,
			}, addonName),
			err: testutil.NewStatusError(".spec.install.olmAllNamespaces is required when .spec.install.type = OLMAllNamespaces"),
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

	for i, tc := range testCases {
		tc := tc // pin
		s.Run(fmt.Sprintf("test case: %d", i), func() {
			err := integration.Client.Create(ctx, tc.addon)

			if err == nil {
				s.Require().NoError(err)

				// clean-up addon
				err = integration.Client.Delete(ctx, tc.addon)
				s.Require().NoError(err)

				err = integration.WaitToBeGone(s.T(), 5*time.Minute, tc.addon)
				s.Require().NoError(err, "wait for Addon to be deleted")
			} else {
				s.Assert().EqualValues(tc.err, err)
			}
		})
	}
}

func (s *integrationTestSuite) TestAddonSpecImmutability() {
	if !testutil.IsWebhookServerEnabled() {
		s.T().Skip("skipping test as webhook server execution is disabled")
	}

	ctx := context.Background()
	addonName := "reference-addon-test-install-spec-immutability"

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
	s.Require().NoError(err)

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		addon := &addonsv1alpha1.Addon{}
		err := integration.Client.Get(ctx, client.ObjectKey{
			Name: addonName,
		}, addon)
		if err != nil {
			return err
		}

		// update field
		addon.Spec.Install.
			OLMOwnNamespace.
			AddonInstallOLMCommon.
			Channel = "beta"

		err = integration.Client.Update(ctx, addon)
		expectedErr := testutil.NewStatusError(".spec.install.type is immutable")

		// explicitly check error type as
		// `Update` can return many different kinds of errors
		if !reflect.DeepEqual(err, expectedErr) {
			return err
		}
		return nil
	})

	s.Require().NoError(err)
	s.addonCleanup(addon, ctx)
}
