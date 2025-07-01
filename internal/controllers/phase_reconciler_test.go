package controllers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/preflight"
	"package-operator.run/internal/testutil"
	"package-operator.run/internal/testutil/managedcachemocks"
	"package-operator.run/internal/testutil/ownerhandlingmocks"
)

var (
	testScheme = runtime.NewScheme()
	errTest    = errors.New("xxx")
)

func init() {
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := corev1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestPhaseReconciler_TeardownPhase_failing_preflight(t *testing.T) {
	t.Parallel()

	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	accessor := &managedcachemocks.AccessorMock{}
	uncachedClient := testutil.NewClient()

	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	preflightChecker := &preflightCheckerMock{}
	r := &phaseReconciler{
		scheme:         scheme,
		accessor:       accessor,
		uncachedClient: uncachedClient,
		ownerStrategy:  ownerStrategy,
		adoptionChecker: &defaultAdoptionChecker{
			ownerStrategy: ownerStrategy,
			scheme:        scheme,
		},
		patcher:          &defaultPatcher{writer: accessor},
		preflightChecker: preflightChecker,
	}
	owner := &phaseObjectOwnerMock{}
	ownerObj := &unstructured.Unstructured{}
	owner.On("ClientObject").Return(ownerObj)
	owner.On("GetStatusRevision").Return(int64(5))

	ownerStrategy.
		On("SetControllerReference", mock.Anything, mock.Anything).
		Return(nil)

	ctx := context.Background()

	preflightChecker.
		On("Check", ctx, ownerObj, mock.Anything).
		Return([]preflight.Violation{{}}, nil)

	done, err := r.TeardownPhase(ctx, owner, corev1alpha1.ObjectSetTemplatePhase{
		Objects: []corev1alpha1.ObjectSetObject{
			{
				Object: unstructured.Unstructured{},
			},
		},
	})
	require.NoError(t, err)
	assert.True(t, done)
}

func TestPhaseReconciler_TeardownPhase(t *testing.T) {
	t.Parallel()

	type prepared struct {
		scheme           *runtime.Scheme
		accessor         *managedcachemocks.AccessorMock
		uncachedClient   *testutil.CtrlClient
		ownerStrategy    *ownerhandlingmocks.OwnerStrategyMock
		preflightChecker *preflightCheckerMock
		owner            *phaseObjectOwnerMock
		ownerObj         *unstructured.Unstructured
		r                *phaseReconciler
	}

	prepare := func() *prepared {
		scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
		accessor := &managedcachemocks.AccessorMock{}
		uncachedClient := testutil.NewClient()
		ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
		preflightChecker := &preflightCheckerMock{}

		r := &phaseReconciler{
			scheme:         scheme,
			accessor:       accessor,
			uncachedClient: uncachedClient,
			ownerStrategy:  ownerStrategy,
			adoptionChecker: &defaultAdoptionChecker{
				ownerStrategy: ownerStrategy,
				scheme:        scheme,
			},
			patcher:          &defaultPatcher{writer: accessor},
			preflightChecker: preflightChecker,
		}

		owner := &phaseObjectOwnerMock{}
		ownerObj := &unstructured.Unstructured{}
		owner.On("ClientObject").Return(ownerObj)
		owner.On("GetStatusRevision").Return(int64(5))

		return &prepared{
			scheme:           scheme,
			accessor:         accessor,
			uncachedClient:   uncachedClient,
			ownerStrategy:    ownerStrategy,
			preflightChecker: preflightChecker,
			owner:            owner,
			ownerObj:         ownerObj,
			r:                r,
		}
	}

	t.Run("already gone", func(t *testing.T) {
		t.Parallel()

		p := prepare()

		p.ownerStrategy.
			On("SetControllerReference", mock.Anything, mock.Anything).
			Return(nil)

		ctx := context.Background()

		p.uncachedClient.
			On("Get", ctx, mock.Anything, mock.Anything, mock.Anything).
			Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))

		p.preflightChecker.
			On("Check", ctx, p.ownerObj, mock.Anything).
			Return([]preflight.Violation{}, nil)

		done, err := p.r.TeardownPhase(ctx, p.owner, corev1alpha1.ObjectSetTemplatePhase{
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: unstructured.Unstructured{},
				},
			},
		})
		require.NoError(t, err)
		assert.True(t, done)
		p.preflightChecker.AssertExpectations(t)
		p.uncachedClient.AssertExpectations(t)
	})

	t.Run("already gone on delete", func(t *testing.T) {
		t.Parallel()

		p := prepare()

		p.preflightChecker.
			On("Check", mock.Anything, mock.Anything, mock.Anything).
			Return([]preflight.Violation{}, nil)

		p.ownerStrategy.
			On("SetControllerReference", mock.Anything, mock.Anything).
			Return(nil)

		currentObj := &unstructured.Unstructured{}
		p.uncachedClient.
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				out := args.Get(2).(*unstructured.Unstructured)
				*out = *currentObj
			}).
			Return(nil)

		p.ownerStrategy.
			On("IsController", p.ownerObj, currentObj).
			Return(true)

		p.accessor.
			On("Delete", mock.Anything, mock.Anything, mock.Anything).
			Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))

		ctx := context.Background()
		done, err := p.r.TeardownPhase(ctx, p.owner, corev1alpha1.ObjectSetTemplatePhase{
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: unstructured.Unstructured{},
				},
			},
		})
		require.NoError(t, err)
		assert.True(t, done)

		p.accessor.AssertExpectations(t)
		p.uncachedClient.AssertExpectations(t)

		// Ensure that IsController was called with currentObj and not desiredObj.
		// If checking desiredObj, IsController will _always_ return true, which could lead to really nasty behavior.
		p.ownerStrategy.AssertCalled(t, "IsController", p.ownerObj, currentObj)
	})

	t.Run("delete waits", func(t *testing.T) {
		t.Parallel()

		// delete returns false first,
		// we are only really done when the object is gone
		// from the apiserver after all finalizers are handled.

		p := prepare()

		p.ownerStrategy.
			On("SetControllerReference", mock.Anything, mock.Anything).
			Return(nil)

		p.preflightChecker.
			On("Check", mock.Anything, mock.Anything, mock.Anything).
			Return([]preflight.Violation{}, nil)

		currentObj := &unstructured.Unstructured{}
		p.uncachedClient.
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				out := args.Get(2).(*unstructured.Unstructured)
				*out = *currentObj
			}).
			Return(nil)

		p.ownerStrategy.
			On("IsController", p.ownerObj, currentObj).
			Return(true)

		p.accessor.
			On("Delete", mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		ctx := context.Background()
		done, err := p.r.TeardownPhase(ctx, p.owner, corev1alpha1.ObjectSetTemplatePhase{
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: unstructured.Unstructured{},
				},
			},
		})
		require.NoError(t, err)
		assert.False(t, done) // wait for delete confirm
		p.uncachedClient.AssertExpectations(t)

		// It's super important that we don't check ownership on desiredObj on accident, because that will always return true.
		p.ownerStrategy.AssertCalled(t, "IsController", p.ownerObj, currentObj)
	})

	t.Run("not controller", func(t *testing.T) {
		t.Parallel()

		p := prepare()

		p.preflightChecker.
			On("Check", mock.Anything, mock.Anything, mock.Anything).
			Return([]preflight.Violation{}, nil)

		p.ownerStrategy.
			On("SetControllerReference", mock.Anything, mock.Anything).
			Return(nil)

		currentObj := &unstructured.Unstructured{}
		p.uncachedClient.
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				out := args.Get(2).(*unstructured.Unstructured)
				*out = *currentObj
			}).
			Return(nil)

		p.ownerStrategy.
			On("IsController", p.ownerObj, currentObj).
			Return(false)
		p.ownerStrategy.
			On("IsOwner", p.ownerObj, currentObj).
			Return(true)

		p.ownerStrategy.
			On("RemoveOwner", p.ownerObj, currentObj).
			Return(false)

		p.accessor.
			On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		ctx := context.Background()
		done, err := p.r.TeardownPhase(ctx, p.owner, corev1alpha1.ObjectSetTemplatePhase{
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: unstructured.Unstructured{},
				},
			},
		})
		require.NoError(t, err)
		assert.True(t, done)
		p.uncachedClient.AssertExpectations(t)

		// It's super important that we don't check ownership on desiredObj on accident, because that will always return true.
		p.ownerStrategy.AssertCalled(t, "IsController", p.ownerObj, currentObj)
		p.ownerStrategy.AssertCalled(t, "IsOwner", p.ownerObj, currentObj)
		p.ownerStrategy.AssertCalled(t, "RemoveOwner", p.ownerObj, currentObj)
		p.accessor.AssertExpectations(t)
	})
}

func TestPhaseReconciler_reconcileObject(t *testing.T) {
	t.Parallel()

	type prepared struct {
		accessor        *managedcachemocks.AccessorMock
		uncachedClient  *testutil.CtrlClient
		ownerStrategy   *ownerhandlingmocks.OwnerStrategyMock
		owner           *phaseObjectOwnerMock
		adoptionChecker *adoptionCheckerMock
		patcher         *patcherMock
		r               *phaseReconciler
	}

	prepare := func() *prepared {
		accessor := &managedcachemocks.AccessorMock{}
		uncachedClient := testutil.NewClient()
		ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
		adoptionChecker := &adoptionCheckerMock{}
		patcher := &patcherMock{}

		r := &phaseReconciler{
			accessor:        accessor,
			uncachedClient:  uncachedClient,
			adoptionChecker: adoptionChecker,
			ownerStrategy:   ownerStrategy,
			patcher:         patcher,
		}

		owner := &phaseObjectOwnerMock{}

		return &prepared{
			accessor:        accessor,
			uncachedClient:  uncachedClient,
			ownerStrategy:   ownerStrategy,
			owner:           owner,
			adoptionChecker: adoptionChecker,
			patcher:         patcher,
			r:               r,
		}
	}

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		p := prepare()

		p.accessor.
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))
		p.uncachedClient.
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))
		p.accessor.
			On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		ctx := context.Background()
		desired := &unstructured.Unstructured{}
		actual, err := p.r.reconcileObject(ctx, p.owner, desired, nil, corev1alpha1.CollisionProtectionPrevent)
		require.NoError(t, err)

		assert.Same(t, desired, actual)
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()

		p := prepare()

		p.owner.On("ClientObject").Return(&unstructured.Unstructured{})
		p.owner.On("GetStatusRevision").Return(int64(3))

		p.adoptionChecker.
			On("Check", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(true, nil)

		p.accessor.
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		p.ownerStrategy.On("ReleaseController", mock.Anything)
		p.ownerStrategy.
			On("SetControllerReference", mock.Anything, mock.Anything).
			Return(nil)
		p.ownerStrategy.
			On("IsController", mock.Anything, mock.Anything).
			Return(true)

		p.patcher.
			On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		p.patcher.
			On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		ctx := context.Background()
		obj := &unstructured.Unstructured{}
		// set owner refs so we don't run into the panic
		obj.SetOwnerReferences([]metav1.OwnerReference{{}})
		actual, err := p.r.reconcileObject(ctx, p.owner, obj, nil, corev1alpha1.CollisionProtectionPrevent)
		require.NoError(t, err)

		p.patcher.AssertExpectations(t)
		p.adoptionChecker.AssertExpectations(t)
		p.ownerStrategy.AssertExpectations(t)
		p.accessor.AssertExpectations(t)

		assert.Equal(t, &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						corev1alpha1.ObjectSetRevisionAnnotation: "3",
					},
					"ownerReferences": []any{
						map[string]any{
							"apiVersion": "",
							"kind":       "",
							"name":       "",
							"uid":        "",
						},
					},
				},
			},
		}, actual)
	})
}

func TestPhaseReconciler_desiredObject(t *testing.T) {
	t.Parallel()

	t.Run("forwardLabels", func(t *testing.T) {
		t.Parallel()

		r := &phaseReconciler{}

		ctx := context.Background()
		owner := &phaseObjectOwnerMock{}
		ownerObj := &unstructured.Unstructured{}
		ownerObj.SetLabels(map[string]string{
			manifestsv1alpha1.PackageLabel:         "pkg-label",
			manifestsv1alpha1.PackageInstanceLabel: "pkg-instance-label",
		})
		owner.On("ClientObject").Return(ownerObj)
		owner.On("GetStatusRevision").Return(int64(5))

		phaseObject := corev1alpha1.ObjectSetObject{
			Object: unstructured.Unstructured{
				Object: map[string]any{"kind": "test"},
			},
		}
		desiredObj := r.desiredObject(ctx, owner, phaseObject)

		assert.Equal(t, &unstructured.Unstructured{
			Object: map[string]any{
				"kind": "test",
				"metadata": map[string]any{
					"annotations": map[string]any{
						corev1alpha1.ObjectSetRevisionAnnotation: "5",
					},
					"labels": map[string]any{
						constants.DynamicCacheLabel:            "True",
						manifestsv1alpha1.PackageLabel:         "pkg-label",
						manifestsv1alpha1.PackageInstanceLabel: "pkg-instance-label",
					},
				},
			},
		}, desiredObj)
	})

	t.Run("defaultNamespace", func(t *testing.T) {
		t.Parallel()

		r := &phaseReconciler{}

		ctx := context.Background()
		owner := &phaseObjectOwnerMock{}
		ownerObj := &unstructured.Unstructured{}
		ownerObj.SetNamespace("my-owner-ns")
		owner.On("ClientObject").Return(ownerObj)
		owner.On("GetStatusRevision").Return(int64(5))

		phaseObject := corev1alpha1.ObjectSetObject{
			Object: unstructured.Unstructured{
				Object: map[string]any{"kind": "test"},
			},
		}
		desiredObj := r.desiredObject(ctx, owner, phaseObject)

		assert.Equal(t, &unstructured.Unstructured{
			Object: map[string]any{
				"kind": "test",
				"metadata": map[string]any{
					"annotations": map[string]any{
						corev1alpha1.ObjectSetRevisionAnnotation: "5",
					},
					"labels": map[string]any{
						constants.DynamicCacheLabel: "True",
					},
					"namespace": "my-owner-ns",
				},
			},
		}, desiredObj)
	})
}

// TODO: refactor and simplify at some point
//
//nolint:maintidx
func Test_defaultAdoptionChecker(t *testing.T) {
	t.Parallel()

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name                string
			mockPrepare         func(*ownerhandlingmocks.OwnerStrategyMock, *phaseObjectOwnerMock)
			object              client.Object
			previous            []PreviousObjectSet
			collisionProtection corev1alpha1.CollisionProtection
			errorAs             error
			needsAdoption       bool
		}{
			{
				// Object is of revision 15, while our current revision is 34.
				// Expect to confirm adoption with no error.
				name: "owned by older revision",
				mockPrepare: func(
					osm *ownerhandlingmocks.OwnerStrategyMock,
					owner *phaseObjectOwnerMock,
				) {
					ownerObj := &unstructured.Unstructured{
						Object: map[string]any{},
					}
					owner.On("ClientObject").Return(ownerObj)
					osm.
						On("IsController", ownerObj, mock.Anything).
						Return(false)
					osm.
						On("IsController", mock.AnythingOfType("*unstructured.Unstructured"), mock.Anything).
						Return(true)
					owner.
						On("GetStatusRevision").Return(int64(34))
				},
				previous: []PreviousObjectSet{
					newPreviousObjectSetMockWithoutRemotes(
						&unstructured.Unstructured{}),
				},
				object: &unstructured.Unstructured{
					Object: map[string]any{
						"metadata": map[string]any{
							"annotations": map[string]any{
								corev1alpha1.ObjectSetRevisionAnnotation: "15",
							},
						},
					},
				},
				needsAdoption: true,
			},
			{
				// Object is already controlled my this owner.
				// ->no op
				name: "already controller",
				mockPrepare: func(
					osm *ownerhandlingmocks.OwnerStrategyMock,
					owner *phaseObjectOwnerMock,
				) {
					ownerObj := &unstructured.Unstructured{
						Object: map[string]any{},
					}
					owner.On("ClientObject").Return(ownerObj)
					osm.
						On("IsController", ownerObj, mock.Anything).
						Return(true)
				},
				object: &unstructured.Unstructured{
					Object: map[string]any{},
				},
				needsAdoption: false,
			},
			{
				// Object is owned by a newer revision than owner.
				name: "owned by newer revision",
				mockPrepare: func(
					osm *ownerhandlingmocks.OwnerStrategyMock,
					owner *phaseObjectOwnerMock,
				) {
					ownerObj := &unstructured.Unstructured{
						Object: map[string]any{},
					}
					owner.On("ClientObject").Return(ownerObj)
					osm.
						On("IsController", ownerObj, mock.Anything).
						Return(false)
					osm.
						On("IsController", mock.AnythingOfType("*unstructured.Unstructured"), mock.Anything).
						Return(true)
					owner.
						On("GetStatusRevision").Return(int64(34))
				},
				previous: []PreviousObjectSet{
					newPreviousObjectSetMockWithoutRemotes(
						&unstructured.Unstructured{}),
				},
				object: &unstructured.Unstructured{
					Object: map[string]any{
						"metadata": map[string]any{
							"annotations": map[string]any{
								corev1alpha1.ObjectSetRevisionAnnotation: "100",
							},
						},
					},
				},
				needsAdoption: false,
			},
			{
				// Object owner is not in previous revision list.
				name: "object not owned by previous revision",
				mockPrepare: func(
					osm *ownerhandlingmocks.OwnerStrategyMock,
					owner *phaseObjectOwnerMock,
				) {
					osm.
						On("IsController", mock.Anything, mock.Anything).
						Return(false)
					ownerObj := &unstructured.Unstructured{
						Object: map[string]any{},
					}
					owner.On("ClientObject").Return(ownerObj)
					owner.On("GetStatusRevision").Return(int64(1))
				},
				previous: []PreviousObjectSet{
					newPreviousObjectSetMockWithoutRemotes(
						&unstructured.Unstructured{}),
				},
				object: &unstructured.Unstructured{
					Object: map[string]any{},
				},
				errorAs:       &ObjectNotOwnedByPreviousRevisionError{},
				needsAdoption: false,
			},
			{
				// both the object and the owner have the same revision number,
				// but the owner is not the same.
				name: "revision collision",
				mockPrepare: func(
					osm *ownerhandlingmocks.OwnerStrategyMock,
					owner *phaseObjectOwnerMock,
				) {
					ownerObj := &unstructured.Unstructured{}
					owner.On("ClientObject").Return(ownerObj)
					osm.
						On("IsController", ownerObj, mock.Anything).
						Return(false)
					osm.
						On("IsController", mock.AnythingOfType("*v1.ConfigMap"), mock.Anything).
						Return(true)
					owner.
						On("GetStatusRevision").Return(int64(100))
				},
				previous: []PreviousObjectSet{
					newPreviousObjectSetMockWithoutRemotes(
						&corev1.ConfigMap{}),
				},
				object: &unstructured.Unstructured{
					Object: map[string]any{
						"metadata": map[string]any{
							"annotations": map[string]any{
								corev1alpha1.ObjectSetRevisionAnnotation: "100",
							},
						},
					},
				},
				errorAs:       &RevisionCollisionError{},
				needsAdoption: false,
			},
			{
				name: "collision protection IfNoController positive",
				mockPrepare: func(
					osm *ownerhandlingmocks.OwnerStrategyMock,
					owner *phaseObjectOwnerMock,
				) {
					ownerObj := &unstructured.Unstructured{}
					owner.On("ClientObject").Return(ownerObj)
					owner.
						On("GetStatusRevision").Return(int64(100))

					osm.
						On("IsController", ownerObj, mock.Anything).
						Return(false)
					osm.
						On("GetController", mock.Anything).
						Return(metav1.OwnerReference{}, false)
				},
				previous: []PreviousObjectSet{
					newPreviousObjectSetMockWithoutRemotes(
						&corev1.ConfigMap{}),
				},
				collisionProtection: corev1alpha1.CollisionProtectionIfNoController,
				object: &unstructured.Unstructured{
					Object: map[string]interface{}{},
				},
				needsAdoption: true,
			},
			{
				// Object owner is not in previous revision list.
				name: "collision protection IfNoController negative",
				mockPrepare: func(
					osm *ownerhandlingmocks.OwnerStrategyMock,
					owner *phaseObjectOwnerMock,
				) {
					osm.
						On("IsController", mock.Anything, mock.Anything).
						Return(false)
					ownerObj := &unstructured.Unstructured{
						Object: map[string]interface{}{},
					}
					osm.
						On("GetController", mock.Anything).
						Return(metav1.OwnerReference{}, true)
					owner.On("ClientObject").Return(ownerObj)
					owner.On("GetStatusRevision").Return(int64(1))
				},
				previous: []PreviousObjectSet{
					newPreviousObjectSetMockWithoutRemotes(
						&unstructured.Unstructured{}),
				},
				collisionProtection: corev1alpha1.CollisionProtectionIfNoController,
				object: &unstructured.Unstructured{
					Object: map[string]interface{}{},
				},
				errorAs:       &ObjectNotOwnedByPreviousRevisionError{},
				needsAdoption: false,
			},
		}

		for i := range tests {
			test := tests[i]

			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				os := &ownerhandlingmocks.OwnerStrategyMock{}
				c := &defaultAdoptionChecker{
					ownerStrategy: os,
					scheme:        testScheme,
				}
				owner := &phaseObjectOwnerMock{}

				test.mockPrepare(os, owner)

				needsAdoption, err := c.Check(
					owner, test.object, test.previous, test.collisionProtection)
				if test.errorAs == nil {
					require.NoError(t, err)
				} else {
					require.ErrorAs(t, err, &test.errorAs) //nolint: testifylint
				}
				assert.Equal(t, test.needsAdoption, needsAdoption)
			})
		}
	})

	t.Run("isControlledByPreviousRevision", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			obj  client.Object
		}{
			{
				name: "ObjectSet",
				obj:  &corev1alpha1.ObjectSet{},
			},
			{
				name: "ClusterObjectSet",
				obj:  &corev1alpha1.ClusterObjectSet{},
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				os := &ownerhandlingmocks.OwnerStrategyMock{}
				ac := &defaultAdoptionChecker{
					scheme:        testScheme,
					ownerStrategy: os,
				}

				os.On("IsController",
					mock.AnythingOfType("*v1alpha1.ObjectSet"),
					mock.Anything,
				).Return(false)

				os.On("IsController",
					mock.AnythingOfType("*v1alpha1.ClusterObjectSet"),
					mock.Anything,
				).Return(false)

				os.On("IsController",
					mock.AnythingOfType("*unstructured.Unstructured"),
					mock.Anything,
				).Return(true)

				previous := &previousObjectSetMock{}
				previous.On("ClientObject").Return(test.obj)
				previous.On("GetStatusRemotePhases").Return([]corev1alpha1.RemotePhaseReference{
					{
						Name: "phase-1",
					},
				})

				obj := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: corev1alpha1.GroupVersion.String(),
								Kind:       "ObjectSetPhase",
								Name:       "phase-1",
								Controller: ptr.To(true),
							},
						},
					},
				}

				isController := ac.isControlledByPreviousRevision(
					obj, []PreviousObjectSet{previous})
				assert.True(t, isController)
			})
		}
	})
}

func Test_defaultPatcher(t *testing.T) {
	t.Parallel()

	t.Run("patchObject_update_metadata", func(t *testing.T) {
		t.Parallel()

		clientMock := testutil.NewClient()
		r := &defaultPatcher{
			writer: clientMock,
		}
		ctx := context.Background()

		var patches []client.Patch
		clientMock.
			On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				patches = append(patches, args.Get(2).(client.Patch))
			}).
			Return(nil)

		// no need to patch anything, all objects are the same
		desiredObj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{
						"my-cool-label": "hans",
					},
				},
			},
		}
		currentObj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"resourceVersion": "123",
					"labels": map[string]any{
						"banana": "hans",
					},
				},
			},
		}
		updatedObj := currentObj.DeepCopy()

		err := r.Patch(ctx, desiredObj, currentObj, updatedObj)
		require.NoError(t, err)

		clientMock.AssertNumberOfCalls(t, "Patch", 1) // only a single PATCH request
		if len(patches) == 1 {
			patch, err := patches[0].Data(updatedObj)
			require.NoError(t, err)

			// ensure patch does NOT contain existing labels
			assert.JSONEq(t,
				`{"metadata":{"labels":{"my-cool-label":"hans"}}}`, string(patch))
		}
	})

	t.Run("patchObject_update_no_metadata", func(t *testing.T) {
		t.Parallel()

		clientMock := testutil.NewClient()
		r := &defaultPatcher{
			writer: clientMock,
		}
		ctx := context.Background()

		var patches []client.Patch
		clientMock.
			On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				patches = append(patches, args.Get(2).(client.Patch))
			}).
			Return(nil)

		desiredObj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{
						"my-cool-label": "hans",
					},
				},
				"spec": map[string]any{
					"key": "val",
				},
			},
		}
		currentObj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"resourceVersion": "123",
					"labels": map[string]any{
						"banana":        "hans", // we don't care about extra labels
						"my-cool-label": "hans",
					},
				},
				"spec": map[string]any{
					"key": "something else",
				},
			},
		}
		updatedObj := currentObj.DeepCopy()
		err := controllerutil.SetControllerReference(&corev1.ConfigMap{}, updatedObj, testScheme)
		require.NoError(t, err)

		err = r.Patch(ctx, desiredObj, currentObj, updatedObj)
		require.NoError(t, err)

		clientMock.AssertNumberOfCalls(t, "Patch", 1) // only a single PATCH request
		if len(patches) == 1 {
			patch, err := patches[0].Data(updatedObj)
			require.NoError(t, err)

			assert.JSONEq(t, `{"metadata":{"labels":{"my-cool-label":"hans"},"ownerReferences":[{"apiVersion":"v1","blockOwnerDeletion":true,"controller":true,"kind":"ConfigMap","name":"","uid":""}]},"spec":{"key":"val"}}`, string(patch)) //nolint: lll
		}
	})

	t.Run("fixFieldManagers", func(t *testing.T) {
		t.Parallel()

		clientMock := testutil.NewClient()
		r := &defaultPatcher{
			writer: clientMock,
		}
		ctx := context.Background()

		clientMock.
			On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		currentObj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name": "test",
				},
			},
		}
		currentObj.SetManagedFields([]metav1.ManagedFieldsEntry{
			{
				Manager:   "package-operator-manager",
				Operation: metav1.ManagedFieldsOperationUpdate,
				FieldsV1:  &metav1.FieldsV1{Raw: []byte(`{}`)},
			},
		})

		err := r.fixFieldManagers(ctx, currentObj)
		require.NoError(t, err)

		clientMock.AssertExpectations(t)
	})

	t.Run("fixFieldManagers_error", func(t *testing.T) {
		t.Parallel()

		clientMock := testutil.NewClient()
		r := &defaultPatcher{
			writer: clientMock,
		}
		ctx := context.Background()

		clientMock.
			On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(errTest)

		currentObj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name": "test",
				},
			},
		}
		currentObj.SetManagedFields([]metav1.ManagedFieldsEntry{
			{
				Manager:   "package-operator-manager",
				Operation: metav1.ManagedFieldsOperationUpdate,
				FieldsV1:  &metav1.FieldsV1{Raw: []byte(`{}`)},
			},
		})

		err := r.fixFieldManagers(ctx, currentObj)
		require.Error(t, err, errTest.Error())

		clientMock.AssertExpectations(t)
	})

	// fixFieldManagers_nowork ensures that fixFieldManagers
	// does not call Patch when no changes are needed.
	t.Run("fixFieldManagers_nowork", func(t *testing.T) {
		t.Parallel()

		clientMock := testutil.NewClient()
		r := &defaultPatcher{writer: clientMock}
		ctx := context.Background()

		currentObj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{"name": "test"},
			},
		}
		currentObj.SetManagedFields([]metav1.ManagedFieldsEntry{{
			Manager:   "package-operator",
			Operation: metav1.ManagedFieldsOperationApply,
			FieldsV1:  &metav1.FieldsV1{Raw: []byte(`{}`)},
		}})

		err := r.fixFieldManagers(ctx, currentObj)
		require.NoError(t, err)

		clientMock.AssertExpectations(t)
	})
}

func Test_mergeKeysFrom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		base, additional map[string]string
		expected         map[string]string
	}{
		{
			name:       "nil base",
			additional: map[string]string{"k": "v"},
			expected:   map[string]string{"k": "v"},
		},
		{
			name:       "overrides",
			base:       map[string]string{"k1": "v", "k2": "v"},
			additional: map[string]string{"k1": "v2"},
			expected:   map[string]string{"k1": "v2", "k2": "v"},
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			r := mergeKeysFrom(test.base, test.additional)
			assert.Equal(t, test.expected, r)
		})
	}
}

func Test_mapConditions(t *testing.T) {
	t.Parallel()

	const (
		reason  = "ChickenSalad"
		message = "Salad made with chicken!"
	)

	tests := []struct {
		name             string
		object           *unstructured.Unstructured
		mappedConditions int
	}{
		{
			name: "no condition observedGeneration",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"generation": int64(9),
					},
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"type":    "Available",
								"status":  "True",
								"reason":  reason,
								"message": message,
							},
							map[string]any{
								"type":   "Other Condition",
								"status": "True",
							},
						},
					},
				},
			},
			mappedConditions: 1,
		},
		{
			name: "observedGeneration outdated",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"generation": int64(9),
					},
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"observedGeneration": 8,
								"type":               "Available",
								"status":             "True",
								"reason":             reason,
								"message":            message,
							},
						},
					},
				},
			},
			mappedConditions: 0,
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			owner := &phaseObjectOwnerMock{}
			ownerObj := &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"generation": int64(4),
					},
				},
			}
			var conditions []metav1.Condition
			owner.On("ClientObject").Return(ownerObj)
			owner.On("GetStatusConditions").Return(&conditions)

			err := mapConditions(ctx, owner, []corev1alpha1.ConditionMapping{
				{
					SourceType:      "Available",
					DestinationType: "my-prefix/Available",
				},
			}, test.object)
			require.NoError(t, err)

			if assert.Len(t, conditions, test.mappedConditions) &&
				test.mappedConditions > 0 {
				assert.Equal(t, metav1.ConditionTrue, conditions[0].Status)
				assert.Equal(t, reason, conditions[0].Reason)
				assert.Equal(t, message, conditions[0].Message)
			}
		})
	}
}

func TestPhaseReconciler_ReconcilePhase_preflightError(t *testing.T) {
	t.Parallel()

	pcm := &preflightCheckerMock{}
	pr := &phaseReconciler{
		scheme:           testScheme,
		preflightChecker: pcm,
	}

	ownerObj := &unstructured.Unstructured{}
	owner := &phaseObjectOwnerMock{}
	owner.On("ClientObject").Return(ownerObj)
	owner.On("GetStatusRevision").Return(int64(12))

	pcm.
		On("Check", mock.Anything, mock.Anything, mock.Anything).
		Return([]preflight.Violation{{}}, nil)

	phase := corev1alpha1.ObjectSetTemplatePhase{
		Objects: []corev1alpha1.ObjectSetObject{
			{
				Object: unstructured.Unstructured{},
			},
		},
	}

	ctx := context.Background()
	_, _, err := pr.ReconcilePhase(
		ctx, owner, phase, nil, nil)
	var pErr *preflight.Error
	require.ErrorAs(t, err, &pErr)
}

func TestIsAdoptionRefusedError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		err    error
		result bool
	}{
		{
			name:   "other error",
			err:    errTest,
			result: false,
		},
		{
			name:   "no error",
			err:    nil,
			result: false,
		},
		{
			name:   "adoption refused",
			err:    &ObjectNotOwnedByPreviousRevisionError{},
			result: true,
		},
		{
			name:   "collision",
			err:    &RevisionCollisionError{},
			result: true,
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			r := IsAdoptionRefusedError(test.err)
			assert.Equal(t, test.result, r)
		})
	}
}

func TestUpdateObjectSetOrPhaseStatusFromError(t *testing.T) {
	t.Parallel()

	t.Run("just returns error", func(t *testing.T) {
		t.Parallel()

		objectSet := &objectSetOrPhaseStub{}

		um := &testUpdateMock{}
		ctx := context.Background()
		res, err := UpdateObjectSetOrPhaseStatusFromError(ctx, objectSet, errTest, um.Update)

		require.EqualError(t, err, errTest.Error())
		assert.True(t, res.IsZero())
		assert.Empty(t, objectSet.GetStatusConditions())
	})

	t.Run("reports preflight error", func(t *testing.T) {
		t.Parallel()

		objectSet := &objectSetOrPhaseStub{}

		um := &testUpdateMock{}

		um.On("Update", mock.Anything).Return(nil)

		ctx := context.Background()
		res, err := UpdateObjectSetOrPhaseStatusFromError(ctx, objectSet, &preflight.Error{}, um.Update)

		require.NoError(t, err)
		assert.Equal(t, DefaultGlobalMissConfigurationRetry, res.RequeueAfter)
		if assert.NotEmpty(t, objectSet.GetStatusConditions()) {
			cond := meta.FindStatusCondition(*objectSet.GetStatusConditions(), corev1alpha1.ObjectSetAvailable)
			assert.Equal(t, "PreflightError", cond.Reason)
		}

		um.AssertExpectations(t)
	})

	t.Run("reports collision error", func(t *testing.T) {
		t.Parallel()

		objectSet := &objectSetOrPhaseStub{}

		um := &testUpdateMock{}

		um.On("Update", mock.Anything).Return(nil)

		ctx := context.Background()
		res, err := UpdateObjectSetOrPhaseStatusFromError(ctx, objectSet, &ObjectNotOwnedByPreviousRevisionError{}, um.Update)

		require.NoError(t, err)
		assert.Equal(t, DefaultGlobalMissConfigurationRetry, res.RequeueAfter)
		if assert.NotEmpty(t, objectSet.GetStatusConditions()) {
			cond := meta.FindStatusCondition(*objectSet.GetStatusConditions(), corev1alpha1.ObjectSetAvailable)
			assert.Equal(t, "CollisionDetected", cond.Reason)
		}

		um.AssertExpectations(t)
	})
}
