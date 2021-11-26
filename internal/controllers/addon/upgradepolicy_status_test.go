package addon

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/ocm"
	"github.com/openshift/addon-operator/internal/ocm/ocmtest"
	"github.com/openshift/addon-operator/internal/testutil"
)

func TestAddonReconciler_handleUpgradePolicyStatusReporting(t *testing.T) {
	t.Run("noop without .spec.upgradePolicy", func(t *testing.T) {
		r := &AddonReconciler{}
		log := testutil.NewLogger(t)

		err := r.handleUpgradePolicyStatusReporting(
			context.Background(),
			log,
			&addonsv1alpha1.Addon{},
		)
		require.NoError(t, err)
	})

	t.Run("noop when upgrade already completed", func(t *testing.T) {
		r := &AddonReconciler{}
		log := testutil.NewLogger(t)

		err := r.handleUpgradePolicyStatusReporting(
			context.Background(),
			log,
			&addonsv1alpha1.Addon{
				Spec: addonsv1alpha1.AddonSpec{
					UpgradePolicy: &addonsv1alpha1.AddonUpgradePolicy{
						ID: "1234",
					},
				},
				Status: addonsv1alpha1.AddonStatus{
					UpgradePolicy: &addonsv1alpha1.AddonUpgradePolicyStatus{
						ID:    "1234",
						Value: addonsv1alpha1.AddonUpgradePolicyValueCompleted,
					},
				},
			},
		)
		require.NoError(t, err)
	})

	t.Run("noop when OCM client is missing", func(t *testing.T) {
		r := &AddonReconciler{}
		log := testutil.NewLogger(t)

		err := r.handleUpgradePolicyStatusReporting(
			context.Background(),
			log,
			&addonsv1alpha1.Addon{
				Spec: addonsv1alpha1.AddonSpec{
					UpgradePolicy: &addonsv1alpha1.AddonUpgradePolicy{
						ID: "1234",
					},
				},
			},
		)
		require.NoError(t, err)
	})

	t.Run("post `started` on new upgradePolicyID", func(t *testing.T) {
		client := testutil.NewClient()
		ocmClient := ocmtest.NewClient()
		r := &AddonReconciler{
			Client:    client,
			ocmClient: ocmClient,
		}
		log := testutil.NewLogger(t)
		addon := &addonsv1alpha1.Addon{
			ObjectMeta: metav1.ObjectMeta{
				Generation: 100,
			},
			Spec: addonsv1alpha1.AddonSpec{
				UpgradePolicy: &addonsv1alpha1.AddonUpgradePolicy{
					ID: "1234",
				},
			},
		}

		ocmClient.
			On("PatchUpgradePolicy", mock.Anything, ocm.UpgradePolicyPatchRequest{
				ID:          "1234",
				Value:       ocm.UpgradePolicyValueStarted,
				Description: "Upgrading addon.",
			}).
			Return(
				ocm.UpgradePolicyPatchResponse{},
				nil,
			)

		client.StatusMock.
			On("Update", mock.Anything,
				mock.AnythingOfType("*v1alpha1.Addon"),
				mock.Anything).
			Return(nil)

		err := r.handleUpgradePolicyStatusReporting(
			context.Background(), log, addon)
		require.NoError(t, err)

		ocmClient.AssertExpectations(t)
		client.AssertExpectations(t)

		if assert.NotNil(t, addon.Status.UpgradePolicy) {
			assert.Equal(t, "1234", addon.Status.UpgradePolicy.ID)
			assert.Equal(t,
				addonsv1alpha1.AddonUpgradePolicyValueStarted,
				addon.Status.UpgradePolicy.Value)
			assert.Equal(t,
				addon.Generation,
				addon.Status.UpgradePolicy.ObservedGeneration)
		}
	})

	t.Run("noop when upgrade started, but Addon not Available", func(t *testing.T) {
		ocmClient := ocmtest.NewClient()
		r := &AddonReconciler{
			ocmClient: ocmClient,
		}
		log := testutil.NewLogger(t)

		err := r.handleUpgradePolicyStatusReporting(
			context.Background(),
			log,
			&addonsv1alpha1.Addon{
				Spec: addonsv1alpha1.AddonSpec{
					UpgradePolicy: &addonsv1alpha1.AddonUpgradePolicy{
						ID: "1234",
					},
				},
				Status: addonsv1alpha1.AddonStatus{
					UpgradePolicy: &addonsv1alpha1.AddonUpgradePolicyStatus{
						ID:    "1234",
						Value: addonsv1alpha1.AddonUpgradePolicyValueStarted,
					},
				},
			},
		)
		require.NoError(t, err)
	})

	t.Run("post `completed` after `started` when Available", func(t *testing.T) {
		client := testutil.NewClient()
		ocmClient := ocmtest.NewClient()
		r := &AddonReconciler{
			Client:    client,
			ocmClient: ocmClient,
		}
		log := testutil.NewLogger(t)
		addon := &addonsv1alpha1.Addon{
			ObjectMeta: metav1.ObjectMeta{
				Generation: 100,
			},
			Spec: addonsv1alpha1.AddonSpec{
				UpgradePolicy: &addonsv1alpha1.AddonUpgradePolicy{
					ID: "1234",
				},
			},
			Status: addonsv1alpha1.AddonStatus{
				Conditions: []metav1.Condition{
					{
						Type:   addonsv1alpha1.AddonOperatorAvailable,
						Status: metav1.ConditionTrue,
					},
				},
				UpgradePolicy: &addonsv1alpha1.AddonUpgradePolicyStatus{
					ID:    "1234",
					Value: addonsv1alpha1.AddonUpgradePolicyValueStarted,
				},
			},
		}

		ocmClient.
			On("PatchUpgradePolicy", mock.Anything, ocm.UpgradePolicyPatchRequest{
				ID:          "1234",
				Value:       ocm.UpgradePolicyValueCompleted,
				Description: "Addon was healthy at least once.",
			}).
			Return(
				ocm.UpgradePolicyPatchResponse{},
				nil,
			)

		client.StatusMock.
			On("Update", mock.Anything,
				mock.AnythingOfType("*v1alpha1.Addon"),
				mock.Anything).
			Return(nil)

		err := r.handleUpgradePolicyStatusReporting(
			context.Background(), log, addon)
		require.NoError(t, err)

		ocmClient.AssertExpectations(t)
		client.AssertExpectations(t)

		if assert.NotNil(t, addon.Status.UpgradePolicy) {
			assert.Equal(t, "1234", addon.Status.UpgradePolicy.ID)
			assert.Equal(t,
				addonsv1alpha1.AddonUpgradePolicyValueCompleted,
				addon.Status.UpgradePolicy.Value)
			assert.Equal(t,
				addon.Generation,
				addon.Status.UpgradePolicy.ObservedGeneration)
		}
	})
}
