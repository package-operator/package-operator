package controllers

import (
	"context"
	"testing"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/testutil"
)

func TestEnsureOperatorGroup(t *testing.T) {
	t.Run("ensures OperatorGroup", func(t *testing.T) {
		addonOwnNamespace := &addonsv1alpha1.Addon{
			ObjectMeta: metav1.ObjectMeta{
				Name: "addon-1",
			},
			Spec: addonsv1alpha1.AddonSpec{
				Install: addonsv1alpha1.AddonInstallSpec{
					Type: addonsv1alpha1.OwnNamespace,
					OwnNamespace: &addonsv1alpha1.AddonInstallOwnNamespace{
						AddonInstallCommon: addonsv1alpha1.AddonInstallCommon{
							Namespace: "addon-system",
						},
					},
				},
			},
		}

		addonAllNamespaces := &addonsv1alpha1.Addon{
			ObjectMeta: metav1.ObjectMeta{
				Name: "addon-1",
			},
			Spec: addonsv1alpha1.AddonSpec{
				Install: addonsv1alpha1.AddonInstallSpec{
					Type: addonsv1alpha1.AllNamespaces,
					AllNamespaces: &addonsv1alpha1.AddonInstallAllNamespaces{
						AddonInstallCommon: addonsv1alpha1.AddonInstallCommon{
							Namespace: "addon-system",
						},
					},
				},
			},
		}

		tests := []struct {
			name                     string
			addon                    *addonsv1alpha1.Addon
			targetNamespace          string
			expectedTargetNamespaces []string
		}{
			{
				name:                     "OwnNamespace",
				addon:                    addonOwnNamespace,
				targetNamespace:          addonOwnNamespace.Spec.Install.OwnNamespace.Namespace,
				expectedTargetNamespaces: []string{addonOwnNamespace.Spec.Install.OwnNamespace.Namespace},
			},
			{
				name:            "AllNamespaces",
				addon:           addonAllNamespaces,
				targetNamespace: addonAllNamespaces.Spec.Install.AllNamespaces.Namespace,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				log := testutil.NewLogger(t)
				c := testutil.NewClient()
				r := AddonReconciler{
					Client: c,
					Scheme: newTestSchemeWithAddonsv1alpha1(),
				}
				addon := test.addon

				// Mock Setup
				c.
					On(
						"Get",
						mock.Anything,
						client.ObjectKey{
							Name:      addon.Name,
							Namespace: test.targetNamespace,
						},
						mock.Anything,
					).
					Return(errors.NewNotFound(schema.GroupResource{}, ""))
				var createdOpeatorGroup *operatorsv1.OperatorGroup
				c.
					On(
						"Create",
						mock.Anything,
						mock.IsType(&operatorsv1.OperatorGroup{}),
						mock.Anything,
					).
					Run(func(args mock.Arguments) {
						createdOpeatorGroup = args.Get(1).(*operatorsv1.OperatorGroup)
					}).
					Return(nil)

				// Test
				ctx := context.Background()
				stop, err := r.ensureOperatorGroup(ctx, log, addon)
				require.NoError(t, err)
				assert.False(t, stop)

				if c.AssertCalled(
					t, "Create",
					mock.Anything,
					mock.IsType(&operatorsv1.OperatorGroup{}),
					mock.Anything,
				) {
					assert.Equal(t, addon.Name, createdOpeatorGroup.Name)
					assert.Equal(t, test.targetNamespace, createdOpeatorGroup.Namespace)

					assert.Equal(t, test.expectedTargetNamespaces, createdOpeatorGroup.Spec.TargetNamespaces)
				}
			})
		}

	})

	t.Run("guards against invalid configuration", func(t *testing.T) {
		tests := []struct {
			name  string
			addon *addonsv1alpha1.Addon
		}{
			{
				name: "ownNamespace is nil",
				addon: &addonsv1alpha1.Addon{
					ObjectMeta: metav1.ObjectMeta{
						Name: "addon-1",
					},
					Spec: addonsv1alpha1.AddonSpec{
						Install: addonsv1alpha1.AddonInstallSpec{
							Type: addonsv1alpha1.OwnNamespace,
						},
					},
				},
			},
			{
				name: "ownNamespace.namespace is empty",
				addon: &addonsv1alpha1.Addon{
					ObjectMeta: metav1.ObjectMeta{
						Name: "addon-1",
					},
					Spec: addonsv1alpha1.AddonSpec{
						Install: addonsv1alpha1.AddonInstallSpec{
							Type:         addonsv1alpha1.OwnNamespace,
							OwnNamespace: &addonsv1alpha1.AddonInstallOwnNamespace{},
						},
					},
				},
			},
			{
				name: "allNamespaces is nil",
				addon: &addonsv1alpha1.Addon{
					ObjectMeta: metav1.ObjectMeta{
						Name: "addon-1",
					},
					Spec: addonsv1alpha1.AddonSpec{
						Install: addonsv1alpha1.AddonInstallSpec{
							Type: addonsv1alpha1.AllNamespaces,
						},
					},
				},
			},
			{
				name: "allNamespaces.namespace is empty",
				addon: &addonsv1alpha1.Addon{
					ObjectMeta: metav1.ObjectMeta{
						Name: "addon-1",
					},
					Spec: addonsv1alpha1.AddonSpec{
						Install: addonsv1alpha1.AddonInstallSpec{
							Type:          addonsv1alpha1.AllNamespaces,
							AllNamespaces: &addonsv1alpha1.AddonInstallAllNamespaces{},
						},
					},
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				log := testutil.NewLogger(t)
				c := testutil.NewClient()
				r := AddonReconciler{
					Client: c,
					Scheme: newTestSchemeWithAddonsv1alpha1(),
				}

				// Mock Setup
				c.StatusMock.
					On(
						"Update",
						mock.Anything,
						mock.IsType(&addonsv1alpha1.Addon{}),
						mock.Anything,
					).
					Return(nil)

				// Test
				ctx := context.Background()
				stop, err := r.ensureOperatorGroup(ctx, log, test.addon)
				require.NoError(t, err)
				assert.True(t, stop)

				c.StatusMock.AssertCalled(
					t, "Update", mock.Anything, test.addon, mock.Anything)

				availableCond := meta.FindStatusCondition(test.addon.Status.Conditions, addonsv1alpha1.Available)
				if assert.NotNil(t, availableCond) {
					assert.Equal(t, metav1.ConditionFalse, availableCond.Status)
					assert.Equal(t, "ConfigurationError", availableCond.Reason)
				}
			})
		}
	})

	t.Run("unsupported install type", func(t *testing.T) {
		addonUnsupported := &addonsv1alpha1.Addon{
			ObjectMeta: metav1.ObjectMeta{
				Name: "addon-1",
			},
			Spec: addonsv1alpha1.AddonSpec{
				Install: addonsv1alpha1.AddonInstallSpec{
					Type: addonsv1alpha1.AddonInstallType("something something"),
				},
			},
		}

		log := testutil.NewLogger(t)
		c := testutil.NewClient()
		r := AddonReconciler{
			Client: c,
			Scheme: newTestSchemeWithAddonsv1alpha1(),
		}

		// Test
		ctx := context.Background()
		stop, err := r.ensureOperatorGroup(ctx, log, addonUnsupported.DeepCopy())
		require.NoError(t, err)
		assert.True(t, stop)

		// indirect sanity check
		// nothing was called on the client and the method signals to stop
	})
}

func TestReconcileOperatorGroup(t *testing.T) {
	operatorGroup := &operatorsv1.OperatorGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testing",
			Namespace: "testing-ns",
		},
		Spec: operatorsv1.OperatorGroupSpec{
			TargetNamespaces: []string{"testing-ns"},
		},
	}

	t.Run("no-op", func(t *testing.T) {
		c := testutil.NewClient()
		r := AddonReconciler{
			Client: c,
			Scheme: newTestSchemeWithAddonsv1alpha1(),
		}

		c.
			On(
				"Get",
				mock.Anything,
				client.ObjectKeyFromObject(operatorGroup),
				mock.IsType(&operatorsv1.OperatorGroup{}),
			).
			Run(func(args mock.Arguments) {
				og := args.Get(2).(*operatorsv1.OperatorGroup)
				operatorGroup.DeepCopyInto(og)
			}).
			Return(nil)

		ctx := context.Background()
		err := r.reconcileOperatorGroup(ctx, operatorGroup.DeepCopy())
		require.NoError(t, err)
	})

	t.Run("update", func(t *testing.T) {
		c := testutil.NewClient()
		r := AddonReconciler{
			Client: c,
			Scheme: newTestSchemeWithAddonsv1alpha1(),
		}

		c.
			On(
				"Get",
				mock.Anything,
				client.ObjectKeyFromObject(operatorGroup),
				mock.IsType(&operatorsv1.OperatorGroup{}),
			).
			Return(nil)

		c.
			On(
				"Update",
				mock.Anything,
				mock.IsType(&operatorsv1.OperatorGroup{}),
				mock.Anything,
			).
			Return(nil)

		ctx := context.Background()
		err := r.reconcileOperatorGroup(ctx, operatorGroup.DeepCopy())
		require.NoError(t, err)

		c.AssertCalled(t,
			"Update",
			mock.Anything,
			mock.IsType(&operatorsv1.OperatorGroup{}),
			mock.Anything,
		)
	})
}
