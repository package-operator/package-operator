package addon

import (
	"context"
	"testing"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/controllers"
	"github.com/openshift/addon-operator/internal/testutil"
)

func TestEnsureSubscription_Adoption(t *testing.T) {
	for name, tc := range map[string]struct {
		MustAdopt  bool
		Strategy   addonsv1alpha1.ResourceAdoptionStrategyType
		AssertFunc func(*testing.T, *operatorsv1alpha1.Subscription, error)
	}{
		"no strategy/no adoption": {
			MustAdopt:  false,
			Strategy:   addonsv1alpha1.ResourceAdoptionStrategyType(""),
			AssertFunc: assertReconciledSubscription,
		},
		"Prevent/no adoption": {
			MustAdopt:  false,
			Strategy:   addonsv1alpha1.ResourceAdoptionPrevent,
			AssertFunc: assertReconciledSubscription,
		},
		"AdoptAll/no adoption": {
			MustAdopt:  false,
			Strategy:   addonsv1alpha1.ResourceAdoptionAdoptAll,
			AssertFunc: assertReconciledSubscription,
		},
		"no strategy/must adopt": {
			MustAdopt:  true,
			Strategy:   addonsv1alpha1.ResourceAdoptionStrategyType(""),
			AssertFunc: assertUnreconciledSubscription,
		},
		"Prevent/must adopt": {
			MustAdopt:  true,
			Strategy:   addonsv1alpha1.ResourceAdoptionPrevent,
			AssertFunc: assertUnreconciledSubscription,
		},
		"AdoptAll/must adopt": {
			MustAdopt:  true,
			Strategy:   addonsv1alpha1.ResourceAdoptionAdoptAll,
			AssertFunc: assertReconciledSubscription,
		},
	} {
		t.Run(name, func(t *testing.T) {
			subscription := testutil.NewTestSubscription()

			c := testutil.NewClient()
			c.On("Get",
				testutil.IsContext,
				testutil.IsObjectKey,
				testutil.IsOperatorsV1Alpha1SubscriptionPtr,
			).Run(func(args mock.Arguments) {
				var sub *operatorsv1alpha1.Subscription

				if tc.MustAdopt {
					sub = testutil.NewTestSubscriptionWithoutOwner()
				} else {
					sub = testutil.NewTestSubscription()
					// Unrelated spec change to force reconciliation
					sub.Spec.Channel = "alpha"
				}

				sub.DeepCopyInto(args.Get(2).(*operatorsv1alpha1.Subscription))
			}).Return(nil)

			if !tc.MustAdopt || (tc.MustAdopt && tc.Strategy == addonsv1alpha1.ResourceAdoptionAdoptAll) {
				c.On("Update",
					testutil.IsContext,
					testutil.IsOperatorsV1Alpha1SubscriptionPtr,
					mock.Anything,
				).Return(nil)
			}

			rec := AddonReconciler{
				Client: c,
				Scheme: testutil.NewTestSchemeWithAddonsv1alpha1(),
			}

			ctx := context.Background()
			reconciledSubscription, err := rec.reconcileSubscription(ctx, subscription.DeepCopy(), tc.Strategy)

			tc.AssertFunc(t, reconciledSubscription, err)
			c.AssertExpectations(t)
		})
	}
}

func assertReconciledSubscription(t *testing.T, sub *operatorsv1alpha1.Subscription, err error) {
	t.Helper()

	assert.NoError(t, err)
	assert.NotNil(t, sub)

}

func assertUnreconciledSubscription(t *testing.T, sub *operatorsv1alpha1.Subscription, err error) {
	t.Helper()

	assert.Error(t, err)
	assert.EqualError(t, err, controllers.ErrNotOwnedByUs.Error())
}
