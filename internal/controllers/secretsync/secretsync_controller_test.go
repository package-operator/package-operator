package secretsync

// import (
// 	"context"
// 	"errors"
// 	"fmt"
// 	"slices"
// 	"strings"
// 	"testing"

// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/mock"
// 	"github.com/stretchr/testify/require"
// 	"golang.org/x/exp/maps"
// 	v1 "k8s.io/api/core/v1"
// 	"k8s.io/apimachinery/pkg/api/meta"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/runtime"
// 	"k8s.io/apimachinery/pkg/types"
// 	ctrl "sigs.k8s.io/controller-runtime"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
// 	"sigs.k8s.io/controller-runtime/pkg/reconcile"

// 	"package-operator.run/apis/core/v1alpha1"
// 	"package-operator.run/internal/constants"
// 	"package-operator.run/internal/testutil"
// )

// var testScheme = runtime.NewScheme()

// func init() {
// 	if err := v1alpha1.AddToScheme(testScheme); err != nil {
// 		panic(err)
// 	}
// 	if err := v1.AddToScheme(testScheme); err != nil {
// 		panic(err)
// 	}
// }

// func newTestError() error {
// 	return errors.New("test error") //nolint:err113
// }

// func newSrcObjectKey() v1alpha1.NamespacedName {
// 	return v1alpha1.NamespacedName{
// 		Name:      "src-secret",
// 		Namespace: "src-ns",
// 	}
// }

// func newDestObjectKeys(n int) []types.NamespacedName {
// 	keys := []types.NamespacedName{}
// 	for i := range n {
// 		keys = append(keys, types.NamespacedName{
// 			Name:      fmt.Sprintf("dest-secret-%d", i),
// 			Namespace: "dest-ns",
// 		})
// 	}
// 	return keys
// }

// func convertTypesNSNToV1Alpha1NSN(in types.NamespacedName) v1alpha1.NamespacedName {
// 	return v1alpha1.NamespacedName{
// 		Namespace: in.Namespace,
// 		Name:      in.Name,
// 	}
// }

// func convertTypesNSNsToV1Alpha1NSNs(in []types.NamespacedName) []v1alpha1.NamespacedName {
// 	out := []v1alpha1.NamespacedName{}
// 	for _, inNSN := range in {
// 		out = append(out, convertTypesNSNToV1Alpha1NSN(inNSN))
// 	}
// 	return out
// }

// func TestSecretSyncController_Reconcile_GetSecretSyncErrors(t *testing.T) {
// 	const secretSyncName = "my-sync"

// 	t.Parallel()

// 	c := testutil.NewClient()
// 	log := ctrl.Log.WithName("secretsync controller test")
// 	controller := NewController(c, log, testScheme, nil)

// 	testErr := newTestError()

// 	c.
// 		On("Get", mock.Anything, client.ObjectKey{
// 			Name: secretSyncName,
// 		}, mock.IsType(&v1alpha1.SecretSync{}), mock.Anything).
// 		Return(testErr)

// 	res, err := controller.Reconcile(context.Background(), reconcile.Request{
// 		NamespacedName: types.NamespacedName{
// 			Name: secretSyncName,
// 		},
// 	})

// 	require.ErrorIs(t, err, testErr)
// 	require.Equal(t, ctrl.Result{}, res)

// 	c.AssertExpectations(t)
// 	c.StatusMock.AssertExpectations(t)
// }

// func TestSecretSyncController_Reconcile_Paused(t *testing.T) {
// 	t.Parallel()

// 	const secretSyncName = "my-sync"

// 	type tcase struct {
// 		name         string
// 		programMocks func(
// 			t *testing.T,
// 			c *testutil.CtrlClient,
// 		)
// 		assertExpectations func(
// 			t *testing.T,
// 			c *testutil.CtrlClient,
// 			res ctrl.Result,
// 			err error,
// 		)
// 	}

// 	programStatusMockUpdateAssertCorrectStatusPaused := func(c *testutil.CtrlClient) {
// 		c.StatusMock.
// 			On("Update", mock.Anything, mock.IsType(&v1alpha1.SecretSync{}), mock.Anything).
// 			Run(func(args mock.Arguments) {
// 				secretSync := args.Get(1).(*v1alpha1.SecretSync)

// 				// Assert that paused condition is true.
// 				presentAndEqual := meta.IsStatusConditionPresentAndEqual(
// 					secretSync.Status.Conditions, v1alpha1.SecretSyncPaused, metav1.ConditionTrue)
// 				assert.True(t, presentAndEqual)

// 				// Assert that phase is "Pause".
// 				assert.Equal(t, v1alpha1.SecretSyncStatusPhasePaused, secretSync.Status.Phase)
// 			}).
// 			Return(nil)
// 	}

// 	programStatusMockUpdateAssertCorrectStatusNotPaused := func(c *testutil.CtrlClient) {
// 		c.StatusMock.
// 			On("Update", mock.Anything, mock.IsType(&v1alpha1.SecretSync{}), mock.Anything).
// 			Run(func(args mock.Arguments) {
// 				secretSync := args.Get(1).(*v1alpha1.SecretSync)

// 				// Assert that paused condition is true.
// 				presentAndEqual := meta.IsStatusConditionPresentAndEqual(
// 					secretSync.Status.Conditions, v1alpha1.SecretSyncPaused, metav1.ConditionFalse)
// 				assert.True(t, presentAndEqual)

// 				// Assert that phase is not "Pause".
// 				assert.NotEqual(t, v1alpha1.SecretSyncStatusPhasePaused, secretSync.Status.Phase)
// 			}).
// 			Return(nil)
// 	}

// 	testErr := newTestError()

// 	tcases := []tcase{
// 		{
// 			name: "On_StaleStatus",
// 			programMocks: func(t *testing.T, c *testutil.CtrlClient) {
// 				t.Helper()

// 				c.
// 					On("Get", mock.Anything, client.ObjectKey{
// 						Name: secretSyncName,
// 					}, mock.IsType(&v1alpha1.SecretSync{}), mock.Anything).
// 					Run(func(args mock.Arguments) {
// 						secretSync := args.Get(2).(*v1alpha1.SecretSync)
// 						*secretSync = v1alpha1.SecretSync{
// 							ObjectMeta: metav1.ObjectMeta{
// 								Name: secretSyncName,
// 							},
// 							Spec: v1alpha1.SecretSyncSpec{
// 								Paused: true,
// 							},
// 						}
// 					}).
// 					Return(nil)

// 				programStatusMockUpdateAssertCorrectStatusPaused(c)
// 			},
// 			assertExpectations: func(t *testing.T, c *testutil.CtrlClient, res ctrl.Result, err error) {
// 				t.Helper()

// 				require.NoError(t, err)
// 				require.Equal(t, ctrl.Result{}, res)

// 				c.AssertExpectations(t)
// 				c.StatusMock.AssertExpectations(t)
// 			},
// 		},
// 		{
// 			name: "On_StaleStatus_RightConditionWrongPhase",
// 			programMocks: func(t *testing.T, c *testutil.CtrlClient) {
// 				t.Helper()

// 				c.
// 					On("Get", mock.Anything, client.ObjectKey{
// 						Name: secretSyncName,
// 					}, mock.IsType(&v1alpha1.SecretSync{}), mock.Anything).
// 					Run(func(args mock.Arguments) {
// 						secretSync := args.Get(2).(*v1alpha1.SecretSync)
// 						*secretSync = v1alpha1.SecretSync{
// 							ObjectMeta: metav1.ObjectMeta{
// 								Name: secretSyncName,
// 							},
// 							Spec: v1alpha1.SecretSyncSpec{
// 								Paused: true,
// 							},
// 							Status: v1alpha1.SecretSyncStatus{
// 								Conditions: []metav1.Condition{
// 									{
// 										Type:    v1alpha1.SecretSyncPaused,
// 										Status:  metav1.ConditionTrue,
// 										Reason:  "TODO",
// 										Message: "TODO",
// 									},
// 								},
// 							},
// 						}
// 					}).
// 					Return(nil)

// 				programStatusMockUpdateAssertCorrectStatusPaused(c)
// 			},
// 			assertExpectations: func(t *testing.T, c *testutil.CtrlClient, res ctrl.Result, err error) {
// 				t.Helper()

// 				require.NoError(t, err)
// 				require.Equal(t, ctrl.Result{}, res)

// 				c.AssertExpectations(t)
// 				c.StatusMock.AssertExpectations(t)
// 			},
// 		},
// 		{
// 			name: "On_StaleStatus_RightPhaseWrongCondition",
// 			programMocks: func(t *testing.T, c *testutil.CtrlClient) {
// 				t.Helper()

// 				c.
// 					On("Get", mock.Anything, client.ObjectKey{
// 						Name: secretSyncName,
// 					}, mock.IsType(&v1alpha1.SecretSync{}), mock.Anything).
// 					Run(func(args mock.Arguments) {
// 						secretSync := args.Get(2).(*v1alpha1.SecretSync)
// 						*secretSync = v1alpha1.SecretSync{
// 							ObjectMeta: metav1.ObjectMeta{
// 								Name: secretSyncName,
// 							},
// 							Spec: v1alpha1.SecretSyncSpec{
// 								Paused: true,
// 							},
// 							Status: v1alpha1.SecretSyncStatus{
// 								Phase: v1alpha1.SecretSyncStatusPhasePaused,
// 							},
// 						}
// 					}).
// 					Return(nil)

// 				programStatusMockUpdateAssertCorrectStatusPaused(c)
// 			},
// 			assertExpectations: func(t *testing.T, c *testutil.CtrlClient, res ctrl.Result, err error) {
// 				t.Helper()

// 				require.NoError(t, err)
// 				require.Equal(t, ctrl.Result{}, res)

// 				c.AssertExpectations(t)
// 				c.StatusMock.AssertExpectations(t)
// 			},
// 		},
// 		{
// 			name: "On_StaleStatus_RightPhaseWrongCondition",
// 			programMocks: func(t *testing.T, c *testutil.CtrlClient) {
// 				t.Helper()

// 				c.
// 					On("Get", mock.Anything, client.ObjectKey{
// 						Name: secretSyncName,
// 					}, mock.IsType(&v1alpha1.SecretSync{}), mock.Anything).
// 					Run(func(args mock.Arguments) {
// 						secretSync := args.Get(2).(*v1alpha1.SecretSync)
// 						*secretSync = v1alpha1.SecretSync{
// 							ObjectMeta: metav1.ObjectMeta{
// 								Name: secretSyncName,
// 							},
// 							Spec: v1alpha1.SecretSyncSpec{
// 								Paused: true,
// 							},
// 							Status: v1alpha1.SecretSyncStatus{
// 								Phase: v1alpha1.SecretSyncStatusPhasePaused,
// 							},
// 						}
// 					}).
// 					Return(nil)

// 				programStatusMockUpdateAssertCorrectStatusPaused(c)
// 			},
// 			assertExpectations: func(t *testing.T, c *testutil.CtrlClient, res ctrl.Result, err error) {
// 				t.Helper()

// 				require.NoError(t, err)
// 				require.Equal(t, ctrl.Result{}, res)

// 				c.AssertExpectations(t)
// 				c.StatusMock.AssertExpectations(t)
// 			},
// 		},
// 		{
// 			name: "On_StaleStatus_UpdateErrors",
// 			programMocks: func(t *testing.T, c *testutil.CtrlClient) {
// 				t.Helper()

// 				c.
// 					On("Get", mock.Anything, client.ObjectKey{
// 						Name: secretSyncName,
// 					}, mock.IsType(&v1alpha1.SecretSync{}), mock.Anything).
// 					Run(func(args mock.Arguments) {
// 						secretSync := args.Get(2).(*v1alpha1.SecretSync)
// 						*secretSync = v1alpha1.SecretSync{
// 							ObjectMeta: metav1.ObjectMeta{
// 								Name: secretSyncName,
// 							},
// 							Spec: v1alpha1.SecretSyncSpec{
// 								Paused: true,
// 							},
// 						}
// 					}).
// 					Return(nil)

// 				c.StatusMock.
// 					On("Update", mock.Anything, mock.IsType(&v1alpha1.SecretSync{}), mock.Anything).
// 					Return(testErr)
// 			},
// 			assertExpectations: func(t *testing.T, c *testutil.CtrlClient, res ctrl.Result, err error) {
// 				t.Helper()

// 				require.ErrorIs(t, err, testErr)
// 				require.Equal(t, ctrl.Result{}, res)

// 				c.AssertExpectations(t)
// 				c.StatusMock.AssertExpectations(t)
// 			},
// 		},
// 		{
// 			name: "On_UpToDateStatus",
// 			programMocks: func(t *testing.T, c *testutil.CtrlClient) {
// 				t.Helper()

// 				c.
// 					On("Get", mock.Anything, client.ObjectKey{
// 						Name: secretSyncName,
// 					}, mock.IsType(&v1alpha1.SecretSync{}), mock.Anything).
// 					Run(func(args mock.Arguments) {
// 						secretSync := args.Get(2).(*v1alpha1.SecretSync)
// 						*secretSync = v1alpha1.SecretSync{
// 							ObjectMeta: metav1.ObjectMeta{
// 								Name: secretSyncName,
// 							},
// 							Spec: v1alpha1.SecretSyncSpec{
// 								Paused: true,
// 							},
// 							Status: v1alpha1.SecretSyncStatus{
// 								Phase: v1alpha1.SecretSyncStatusPhasePaused,
// 								Conditions: []metav1.Condition{
// 									{
// 										Type:    v1alpha1.SecretSyncPaused,
// 										Status:  metav1.ConditionTrue,
// 										Reason:  "TODO",
// 										Message: "TODO",
// 									},
// 								},
// 							},
// 						}
// 					}).
// 					Return(nil)
// 			},
// 			assertExpectations: func(t *testing.T, c *testutil.CtrlClient, res ctrl.Result, err error) {
// 				t.Helper()

// 				require.NoError(t, err)
// 				require.Equal(t, ctrl.Result{}, res)

// 				c.AssertExpectations(t)
// 				c.StatusMock.AssertExpectations(t)
// 			},
// 		},
// 		{
// 			name: "Off_StaleStatus",
// 			programMocks: func(t *testing.T, c *testutil.CtrlClient) {
// 				t.Helper()

// 				srcRef := newSrcObjectKey()

// 				c.
// 					On("Get", mock.Anything, client.ObjectKey{
// 						Name: secretSyncName,
// 					}, mock.IsType(&v1alpha1.SecretSync{}), mock.Anything).
// 					Run(func(args mock.Arguments) {
// 						secretSync := args.Get(2).(*v1alpha1.SecretSync)
// 						*secretSync = v1alpha1.SecretSync{
// 							ObjectMeta: metav1.ObjectMeta{
// 								Name: secretSyncName,
// 							},
// 							Spec: v1alpha1.SecretSyncSpec{
// 								Paused: false,
// 								Src:    srcRef,
// 							},
// 							Status: v1alpha1.SecretSyncStatus{
// 								Phase: v1alpha1.SecretSyncStatusPhasePaused,
// 								Conditions: []metav1.Condition{
// 									{
// 										Type:    v1alpha1.SecretSyncPaused,
// 										Status:  metav1.ConditionTrue,
// 										Reason:  "TODO",
// 										Message: "TODO",
// 									},
// 								},
// 							},
// 						}
// 					}).
// 					Return(nil)

// 				c.
// 					On("Get", mock.Anything, types.NamespacedName{
// 						Namespace: srcRef.Namespace,
// 						Name:      srcRef.Name,
// 					}, mock.IsType(&v1.Secret{}), mock.Anything).
// 					Return(nil)

// 				programStatusMockUpdateAssertCorrectStatusNotPaused(c)
// 			},
// 			assertExpectations: func(t *testing.T, c *testutil.CtrlClient, res ctrl.Result, err error) {
// 				t.Helper()

// 				require.NoError(t, err)
// 				require.Equal(t, ctrl.Result{}, res)

// 				c.AssertExpectations(t)
// 				c.StatusMock.AssertExpectations(t)
// 			},
// 		},
// 	}

// 	for _, tcase := range tcases {
// 		t.Run(tcase.name, func(t *testing.T) {
// 			t.Parallel()

// 			c := testutil.NewClient()
// 			log := ctrl.Log.WithName("secretsync controller test")
// 			controller := NewController(c, log, testScheme, nil)

// 			tcase.programMocks(t, c)

// 			res, err := controller.Reconcile(context.Background(), reconcile.Request{
// 				NamespacedName: types.NamespacedName{
// 					Name: secretSyncName,
// 				},
// 			})

// 			tcase.assertExpectations(t, c, res, err)
// 		})
// 	}
// }

// func TestSecretSyncController_Reconcile_Full(t *testing.T) {
// 	const secretSyncName = "my-sync"

// 	t.Parallel()

// 	c := testutil.NewClient()
// 	log := ctrl.Log.WithName("secretsync controller test")
// 	controller := NewController(c, log, testScheme, nil)

// 	srcSecretRef := newSrcObjectKey()
// 	secretTemplate := v1.Secret{
// 		StringData: map[string]string{
// 			"foo": "bar",
// 		},
// 	}
// 	destSecretRefs := newDestObjectKeys(5)
// 	destSecretRefsPatched := map[client.ObjectKey]struct{}{}

// 	c.
// 		On("Get", mock.Anything, client.ObjectKey{
// 			Name: secretSyncName,
// 		}, mock.IsType(&v1alpha1.SecretSync{}), mock.Anything).
// 		Run(func(args mock.Arguments) {
// 			secretSync := args.Get(2).(*v1alpha1.SecretSync)
// 			*secretSync = v1alpha1.SecretSync{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name: secretSyncName,
// 				},
// 				Spec: v1alpha1.SecretSyncSpec{
// 					Paused: false,
// 					Strategy: v1alpha1.SecretSyncStrategy{
// 						Watch: &v1alpha1.SecretSyncStrategyWatch{},
// 					},
// 					Src:  srcSecretRef,
// 					Dest: convertTypesNSNsToV1Alpha1NSNs(destSecretRefs),
// 				},
// 			}
// 		}).
// 		Return(nil)

// 	c.
// 		On("Get", mock.Anything, client.ObjectKey{
// 			Name:      srcSecretRef.Name,
// 			Namespace: srcSecretRef.Namespace,
// 		}, mock.IsType(&v1.Secret{}), mock.Anything).
// 		Run(func(args mock.Arguments) {
// 			key := args.Get(1).(client.ObjectKey)
// 			assert.Equal(t, client.ObjectKey{
// 				Namespace: srcSecretRef.Namespace,
// 				Name:      srcSecretRef.Name,
// 			}, key)

// 			secret := args.Get(2).(*v1.Secret)
// 			secretTemplate.DeepCopyInto(secret)
// 			secret.ObjectMeta = metav1.ObjectMeta{
// 				Namespace: srcSecretRef.Namespace,
// 				Name:      srcSecretRef.Name,
// 			}
// 		}).
// 		Return(nil)

// 	c.
// 		On("Patch",
// 			mock.Anything,
// 			mock.IsType(&v1.Secret{}),
// 			mock.AnythingOfType("client.applyPatch"), // This is part of a correct SSA.
// 			mock.IsType([]client.PatchOption{})).
// 		Run(func(args mock.Arguments) {
// 			// Assert correct usage of Server Side Apply (SSA).
// 			opts := args.Get(3).([]client.PatchOption)
// 			assert.Contains(t, opts, client.ForceOwnership)
// 			assert.Contains(t, opts, client.FieldOwner(constants.FieldOwner))

// 			secret := args.Get(1).(*v1.Secret)
// 			secretKey := client.ObjectKeyFromObject(secret)
// 			require.NotContains(t, destSecretRefsPatched, secretKey)
// 			destSecretRefsPatched[secretKey] = struct{}{}

// 			// Asssert that secret data is equal to source secret data.
// 			actual := secret.DeepCopy()
// 			actual.ObjectMeta = metav1.ObjectMeta{}
// 			assert.Equal(t, &secretTemplate, actual)
// 		}).
// 		Return(nil)

// 	c.StatusMock.
// 		On("Update", mock.Anything, mock.IsType(&v1alpha1.SecretSync{}), mock.Anything).
// 		Run(func(_ mock.Arguments) {
// 			// s := args.Get(2).(*v1alpha1.SecretSync)
// 			// assert.True(t, meta.IsStatusConditionTrue())
// 		}).
// 		Return(nil)

// 	res, err := controller.Reconcile(context.Background(), reconcile.Request{
// 		NamespacedName: types.NamespacedName{
// 			Name: "my-sync",
// 		},
// 	})

// 	require.NoError(t, err)
// 	require.Equal(t, ctrl.Result{}, res)

// 	c.AssertExpectations(t)
// 	c.StatusMock.AssertExpectations(t)

// 	// Assert that all destination secrets have been patched.
// 	assert.Len(t, destSecretRefsPatched, len(destSecretRefs))
// 	patchedRefs := maps.Keys[map[types.NamespacedName]struct{}](destSecretRefsPatched)
// 	slices.SortStableFunc[[]types.NamespacedName](patchedRefs, func(a, b types.NamespacedName) int {
// 		return strings.Compare(a.String(), b.String())
// 	})
// 	assert.Equal(t, destSecretRefs, patchedRefs)
// }

// /*
// 	test matrix
// 	ss status phase/conds uptodate/outdated
// 	paused/unpaused
// 	src exists/missing
// 	dest exist/missing
// 	deletion
// */
