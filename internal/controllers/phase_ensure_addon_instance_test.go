package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/testutil"
)

func TestEnsureAddonInstance(t *testing.T) {
	t.Run("ensures AddonInstance", func(t *testing.T) {
		addonInstance := &addonsv1alpha1.AddonInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      addonsv1alpha1.DefaultAddonInstanceName,
				Namespace: "addon-system",
			},
			Spec: addonsv1alpha1.AddonInstanceSpec{
				HeartbeatUpdatePeriod: addonsv1alpha1.DefaultAddonInstanceHeartbeatUpdatePeriod,
			},
		}

		addonOwnNamespace := &addonsv1alpha1.Addon{
			ObjectMeta: metav1.ObjectMeta{
				Name: "addon-1",
			},
			Spec: addonsv1alpha1.AddonSpec{
				Install: addonsv1alpha1.AddonInstallSpec{
					Type: addonsv1alpha1.OLMOwnNamespace,
					OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
						AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
							CatalogSourceImage: "quay.io/osd-addons/test:sha256:04864220677b2ed6244f2e0d421166df908986700647595ffdb6fd9ca4e5098a",
							Namespace:          "addon-system",
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
					Type: addonsv1alpha1.OLMAllNamespaces,
					OLMAllNamespaces: &addonsv1alpha1.AddonInstallOLMAllNamespaces{
						AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
							CatalogSourceImage: "quay.io/osd-addons/test:sha256:04864220677b2ed6244f2e0d421166df908986700647595ffdb6fd9ca4e5098a",
							Namespace:          "addon-system",
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
				targetNamespace:          addonOwnNamespace.Spec.Install.OLMOwnNamespace.Namespace,
				expectedTargetNamespaces: []string{addonOwnNamespace.Spec.Install.OLMOwnNamespace.Namespace},
			},
			{
				name:            "AllNamespaces",
				addon:           addonAllNamespaces,
				targetNamespace: addonAllNamespaces.Spec.Install.OLMAllNamespaces.Namespace,
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
				var createdAddonInstance *addonsv1alpha1.AddonInstance
				c.
					On(
						"Create",
						mock.Anything,
						mock.IsType(&addonsv1alpha1.AddonInstance{}),
						mock.Anything,
					).
					Run(func(args mock.Arguments) {
						createdAddonInstance = args.Get(1).(*addonsv1alpha1.AddonInstance)
					}).
					Return(nil)

				c.
					On(
						"Get",
						mock.Anything,
						client.ObjectKeyFromObject(addonInstance),
						mock.IsType(&addonsv1alpha1.AddonInstance{}),
					).
					Return(nil)

				c.
					On(
						"Update",
						mock.Anything,
						mock.IsType(&addonsv1alpha1.AddonInstance{}),
						mock.Anything,
					).
					Run(func(args mock.Arguments) {
						createdAddonInstance = args.Get(1).(*addonsv1alpha1.AddonInstance)
					}).
					Return(nil)

				// Test
				ctx := context.Background()
				err := r.ensureAddonInstance(ctx, log, addon)
				require.NoError(t, err)

				assert.Equal(t, addonsv1alpha1.DefaultAddonInstanceName, createdAddonInstance.Name)
				assert.Equal(t, test.targetNamespace, createdAddonInstance.Namespace)
				assert.Equal(t, addonsv1alpha1.DefaultAddonInstanceHeartbeatUpdatePeriod, createdAddonInstance.Spec.HeartbeatUpdatePeriod)
			})
		}
	})

	t.Run("gracefully handles invalid configuration", func(t *testing.T) {
		tests := []struct {
			name  string
			addon *addonsv1alpha1.Addon
		}{
			{
				name: "install.type is unsupported",
				addon: &addonsv1alpha1.Addon{
					ObjectMeta: metav1.ObjectMeta{
						Name: "addon-1",
					},
					Spec: addonsv1alpha1.AddonSpec{
						Install: addonsv1alpha1.AddonInstallSpec{
							Type: addonsv1alpha1.AddonInstallType("random"),
						},
					},
				},
			},
			{
				name: "ownNamespace is nil",
				addon: &addonsv1alpha1.Addon{
					ObjectMeta: metav1.ObjectMeta{
						Name: "addon-1",
					},
					Spec: addonsv1alpha1.AddonSpec{
						Install: addonsv1alpha1.AddonInstallSpec{
							Type: addonsv1alpha1.OLMOwnNamespace,
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
							Type:            addonsv1alpha1.OLMOwnNamespace,
							OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{},
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
							Type: addonsv1alpha1.OLMAllNamespaces,
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
							Type:             addonsv1alpha1.OLMAllNamespaces,
							OLMAllNamespaces: &addonsv1alpha1.AddonInstallOLMAllNamespaces{},
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
				err := r.ensureAddonInstance(ctx, log, test.addon)
				require.EqualError(t, err, "failed to create addonInstance due to misconfigured install.spec.type")
			})
		}
	})
}

func TestReconcileAddonInstance(t *testing.T) {
	addonInstance := &addonsv1alpha1.AddonInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addonsv1alpha1.DefaultAddonInstanceName,
			Namespace: "test",
		},
		Spec: addonsv1alpha1.AddonInstanceSpec{
			HeartbeatUpdatePeriod: addonsv1alpha1.DefaultAddonInstanceHeartbeatUpdatePeriod,
		},
	}

	t.Run("no addoninstance", func(t *testing.T) {
		c := testutil.NewClient()
		r := AddonReconciler{
			Client: c,
			Scheme: newTestSchemeWithAddonsv1alpha1(),
		}

		c.
			On(
				"Get",
				mock.Anything,
				client.ObjectKeyFromObject(addonInstance),
				mock.IsType(&addonsv1alpha1.AddonInstance{}),
			).
			Run(func(args mock.Arguments) {
				fetchedAddonInstance := args.Get(2).(*addonsv1alpha1.AddonInstance)
				addonInstance.DeepCopyInto(fetchedAddonInstance)
			}).
			Return(nil)

		ctx := context.Background()
		err := r.reconcileAddonInstance(ctx, addonInstance.DeepCopy())
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
				client.ObjectKeyFromObject(addonInstance),
				mock.IsType(&addonsv1alpha1.AddonInstance{}),
			).
			Return(nil)

		c.
			On(
				"Update",
				mock.Anything,
				mock.IsType(&addonsv1alpha1.AddonInstance{}),
				mock.Anything,
			).
			Return(nil)

		ctx := context.Background()
		err := r.reconcileAddonInstance(ctx, addonInstance.DeepCopy())
		require.NoError(t, err)

		c.AssertCalled(t,
			"Update",
			mock.Anything,
			mock.IsType(&addonsv1alpha1.AddonInstance{}),
			mock.Anything,
		)
	})
}
