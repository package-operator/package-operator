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
)

var testScheme = runtime.NewScheme()

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
	dynamicCache := &dynamicCacheMock{}
	ownerStrategy := &ownerStrategyMock{}
	preflightChecker := &preflightCheckerMock{}
	r := &PhaseReconciler{
		dynamicCache:     dynamicCache,
		ownerStrategy:    ownerStrategy,
		preflightChecker: preflightChecker,
	}
	owner := &phaseObjectOwnerMock{}
	ownerObj := &unstructured.Unstructured{}
	owner.On("ClientObject").Return(ownerObj)
	owner.On("GetRevision").Return(int64(5))

	ownerStrategy.
		On("SetControllerReference", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	dynamicCache.
		On("Watch", mock.Anything, ownerObj, mock.Anything).
		Return(nil)

	dynamicCache.
		On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))

	preflightChecker.
		On("Check", mock.Anything, mock.Anything, mock.Anything).
		Return([]preflight.Violation{{}}, nil)

	ctx := context.Background()
	done, err := r.TeardownPhase(ctx, owner, corev1alpha1.ObjectSetTemplatePhase{
		Objects: []corev1alpha1.ObjectSetObject{
			{
				Object: unstructured.Unstructured{},
			},
		},
	})
	require.NoError(t, err)
	assert.True(t, done)
	dynamicCache.AssertNotCalled(t, "Watch", mock.Anything, ownerObj, mock.Anything)
}

func TestPhaseReconciler_TeardownPhase(t *testing.T) {
	t.Parallel()
	t.Run("already gone", func(t *testing.T) {
		t.Parallel()
		dynamicCache := &dynamicCacheMock{}
		uncachedClient := testutil.NewClient()
		ownerStrategy := &ownerStrategyMock{}
		preflightChecker := &preflightCheckerMock{}
		r := &PhaseReconciler{
			dynamicCache:     dynamicCache,
			uncachedClient:   uncachedClient,
			ownerStrategy:    ownerStrategy,
			preflightChecker: preflightChecker,
		}
		owner := &phaseObjectOwnerMock{}
		ownerObj := &unstructured.Unstructured{}
		owner.On("ClientObject").Return(ownerObj)
		owner.On("GetRevision").Return(int64(5))

		ownerStrategy.
			On("SetControllerReference", mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		dynamicCache.
			On("Watch", mock.Anything, ownerObj, mock.Anything).
			Return(nil)

		uncachedClient.
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))

		preflightChecker.
			On("Check", mock.Anything, mock.Anything, mock.Anything).
			Return([]preflight.Violation{}, nil)

		ctx := context.Background()
		done, err := r.TeardownPhase(ctx, owner, corev1alpha1.ObjectSetTemplatePhase{
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: unstructured.Unstructured{},
				},
			},
		})
		require.NoError(t, err)
		assert.True(t, done)
		dynamicCache.AssertCalled(t, "Watch", mock.Anything, ownerObj, mock.Anything)
		uncachedClient.AssertExpectations(t)
	})

	t.Run("already gone on delete", func(t *testing.T) {
		t.Parallel()
		testClient := testutil.NewClient()
		dynamicCache := &dynamicCacheMock{}
		uncachedClient := testutil.NewClient()
		ownerStrategy := &ownerStrategyMock{}
		preflightChecker := &preflightCheckerMock{}
		r := &PhaseReconciler{
			writer:           testClient,
			dynamicCache:     dynamicCache,
			uncachedClient:   uncachedClient,
			ownerStrategy:    ownerStrategy,
			preflightChecker: preflightChecker,
		}
		owner := &phaseObjectOwnerMock{}
		ownerObj := &unstructured.Unstructured{}
		owner.On("ClientObject").Return(ownerObj)
		owner.On("GetRevision").Return(int64(5))

		preflightChecker.
			On("Check", mock.Anything, mock.Anything, mock.Anything).
			Return([]preflight.Violation{}, nil)

		ownerStrategy.
			On("SetControllerReference", mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		dynamicCache.
			On("Watch", mock.Anything, ownerObj, mock.Anything).
			Return(nil)
		currentObj := &unstructured.Unstructured{}
		uncachedClient.
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				out := args.Get(2).(*unstructured.Unstructured)
				*out = *currentObj
			}).
			Return(nil)

		ownerStrategy.
			On("IsController", ownerObj, currentObj).
			Return(true)

		testClient.
			On("Delete", mock.Anything, mock.Anything, mock.Anything).
			Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))

		ctx := context.Background()
		done, err := r.TeardownPhase(ctx, owner, corev1alpha1.ObjectSetTemplatePhase{
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: unstructured.Unstructured{},
				},
			},
		})
		require.NoError(t, err)
		assert.True(t, done)
		dynamicCache.AssertCalled(t, "Watch", mock.Anything, ownerObj, mock.Anything)
		uncachedClient.AssertExpectations(t)

		// Ensure that IsController was called with currentObj and not desiredObj.
		// If checking desiredObj, IsController will _always_ return true, which could lead to really nasty behavior.
		ownerStrategy.AssertCalled(t, "IsController", ownerObj, currentObj)
	})

	t.Run("delete waits", func(t *testing.T) {
		t.Parallel()
		// delete returns false first,
		// we are only really done when the object is gone
		// from the apiserver after all finalizers are handled.
		testClient := testutil.NewClient()
		dynamicCache := &dynamicCacheMock{}
		uncachedClient := testutil.NewClient()
		ownerStrategy := &ownerStrategyMock{}
		preflightChecker := &preflightCheckerMock{}

		r := &PhaseReconciler{
			writer:           testClient,
			dynamicCache:     dynamicCache,
			uncachedClient:   uncachedClient,
			ownerStrategy:    ownerStrategy,
			preflightChecker: preflightChecker,
		}

		owner := &phaseObjectOwnerMock{}
		ownerObj := &unstructured.Unstructured{}
		owner.On("ClientObject").Return(ownerObj)
		owner.On("GetRevision").Return(int64(5))

		ownerStrategy.
			On("SetControllerReference", mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		preflightChecker.
			On("Check", mock.Anything, mock.Anything, mock.Anything).
			Return([]preflight.Violation{}, nil)

		dynamicCache.
			On("Watch", mock.Anything, ownerObj, mock.Anything).
			Return(nil)
		currentObj := &unstructured.Unstructured{}
		uncachedClient.
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				out := args.Get(2).(*unstructured.Unstructured)
				*out = *currentObj
			}).
			Return(nil)

		ownerStrategy.
			On("IsController", ownerObj, currentObj).
			Return(true)

		testClient.
			On("Delete", mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		ctx := context.Background()
		done, err := r.TeardownPhase(ctx, owner, corev1alpha1.ObjectSetTemplatePhase{
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: unstructured.Unstructured{},
				},
			},
		})
		require.NoError(t, err)
		assert.False(t, done) // wait for delete confirm
		dynamicCache.AssertCalled(t, "Watch", mock.Anything, ownerObj, mock.Anything)
		uncachedClient.AssertExpectations(t)

		// It's super important that we don't check ownership on desiredObj on accident, because that will always return true.
		ownerStrategy.AssertCalled(t, "IsController", ownerObj, currentObj)
	})

	t.Run("not controller", func(t *testing.T) {
		t.Parallel()

		dynamicCache := &dynamicCacheMock{}
		uncachedClient := testutil.NewClient()
		ownerStrategy := &ownerStrategyMock{}
		testClient := testutil.NewClient()
		preflightChecker := &preflightCheckerMock{}
		r := &PhaseReconciler{
			dynamicCache:     dynamicCache,
			uncachedClient:   uncachedClient,
			ownerStrategy:    ownerStrategy,
			writer:           testClient,
			preflightChecker: preflightChecker,
		}

		owner := &phaseObjectOwnerMock{}
		ownerObj := &unstructured.Unstructured{}
		owner.On("ClientObject").Return(ownerObj)
		owner.On("GetRevision").Return(int64(5))

		preflightChecker.
			On("Check", mock.Anything, mock.Anything, mock.Anything).
			Return([]preflight.Violation{}, nil)

		ownerStrategy.
			On("SetControllerReference", mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		dynamicCache.
			On("Watch", mock.Anything, ownerObj, mock.Anything).
			Return(nil)
		currentObj := &unstructured.Unstructured{}
		uncachedClient.
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				out := args.Get(2).(*unstructured.Unstructured)
				*out = *currentObj
			}).
			Return(nil)

		ownerStrategy.
			On("IsController", ownerObj, currentObj).
			Return(false)
		ownerStrategy.
			On("IsOwner", ownerObj, currentObj).
			Return(true)

		ownerStrategy.
			On("RemoveOwner", ownerObj, currentObj).
			Return(false)

		testClient.
			On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		ctx := context.Background()
		done, err := r.TeardownPhase(ctx, owner, corev1alpha1.ObjectSetTemplatePhase{
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: unstructured.Unstructured{},
				},
			},
		})
		require.NoError(t, err)
		assert.True(t, done)
		dynamicCache.AssertCalled(t, "Watch", mock.Anything, ownerObj, mock.Anything)
		uncachedClient.AssertExpectations(t)

		// It's super important that we don't check ownership on desiredObj on accident, because that will always return true.
		ownerStrategy.AssertCalled(t, "IsController", ownerObj, currentObj)
		ownerStrategy.AssertCalled(t, "IsOwner", ownerObj, currentObj)
		ownerStrategy.AssertCalled(t, "RemoveOwner", ownerObj, currentObj)
		testClient.AssertExpectations(t)
	})
}

func TestPhaseReconciler_reconcileObject_create(t *testing.T) {
	t.Parallel()

	testClient := testutil.NewClient()
	dynamicCacheMock := &dynamicCacheMock{}
	clientMock := &testutil.CtrlClient{}
	r := &PhaseReconciler{
		writer:         testClient,
		dynamicCache:   dynamicCacheMock,
		uncachedClient: clientMock,
	}
	owner := &phaseObjectOwnerMock{}

	dynamicCacheMock.
		On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))
	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))
	testClient.
		On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	ctx := context.Background()
	desired := &unstructured.Unstructured{}
	actual, err := r.reconcileObject(ctx, owner, desired, nil, corev1alpha1.CollisionProtectionPrevent)
	require.NoError(t, err)

	assert.Same(t, desired, actual)
}

func TestPhaseReconciler_reconcileObject_update(t *testing.T) {
	t.Parallel()

	testClient := testutil.NewClient()
	dynamicCacheMock := &dynamicCacheMock{}
	acMock := &adoptionCheckerMock{}
	ownerStrategy := &ownerStrategyMock{}
	patcher := &patcherMock{}
	r := &PhaseReconciler{
		writer:          testClient,
		dynamicCache:    dynamicCacheMock,
		adoptionChecker: acMock,
		ownerStrategy:   ownerStrategy,
		patcher:         patcher,
	}
	owner := &phaseObjectOwnerMock{}
	owner.On("ClientObject").Return(&unstructured.Unstructured{})
	owner.On("GetRevision").Return(int64(3))

	acMock.
		On("Check", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)

	dynamicCacheMock.
		On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	ownerStrategy.On("ReleaseController", mock.Anything)
	ownerStrategy.
		On("SetControllerReference", mock.Anything, mock.Anything).
		Return(nil)
	ownerStrategy.
		On("IsController", mock.Anything, mock.Anything).
		Return(true)
	ownerStrategy.
		On("OwnerPatch", mock.Anything).
		Return([]byte(nil), nil)

	testClient.
		On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	patcher.
		On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	ctx := context.Background()
	obj := &unstructured.Unstructured{}
	// set owner refs so we don't run into the panic
	obj.SetOwnerReferences([]metav1.OwnerReference{{}})
	actual, err := r.reconcileObject(ctx, owner, obj, nil, corev1alpha1.CollisionProtectionPrevent)
	require.NoError(t, err)

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
}

func TestPhaseReconciler_desiredObject(t *testing.T) {
	t.Parallel()

	os := &ownerStrategyMock{}
	r := &PhaseReconciler{
		ownerStrategy: os,
	}

	os.On("SetControllerReference",
		mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	ctx := context.Background()
	owner := &phaseObjectOwnerMock{}
	ownerObj := &unstructured.Unstructured{}
	ownerObj.SetLabels(map[string]string{
		manifestsv1alpha1.PackageLabel:         "pkg-label",
		manifestsv1alpha1.PackageInstanceLabel: "pkg-instance-label",
	})
	owner.On("ClientObject").Return(ownerObj)
	owner.On("GetRevision").Return(int64(5))

	phaseObject := corev1alpha1.ObjectSetObject{
		Object: unstructured.Unstructured{
			Object: map[string]any{"kind": "test"},
		},
	}
	desiredObj, err := r.desiredObject(ctx, owner, phaseObject)
	require.NoError(t, err)

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
}

func TestPhaseReconciler_desiredObject_defaultsNamespace(t *testing.T) {
	t.Parallel()

	os := &ownerStrategyMock{}
	r := &PhaseReconciler{
		ownerStrategy: os,
	}

	os.On("SetControllerReference",
		mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	ctx := context.Background()
	owner := &phaseObjectOwnerMock{}
	ownerObj := &unstructured.Unstructured{}
	ownerObj.SetNamespace("my-owner-ns")
	owner.On("ClientObject").Return(ownerObj)
	owner.On("GetRevision").Return(int64(5))

	phaseObject := corev1alpha1.ObjectSetObject{
		Object: unstructured.Unstructured{
			Object: map[string]any{"kind": "test"},
		},
	}
	desiredObj, err := r.desiredObject(ctx, owner, phaseObject)
	require.NoError(t, err)

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
}

func Test_defaultAdoptionChecker_Check(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		mockPrepare         func(*ownerStrategyMock, *phaseObjectOwnerMock)
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
				osm *ownerStrategyMock,
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
					On("GetRevision").Return(int64(34))
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
				osm *ownerStrategyMock,
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
				osm *ownerStrategyMock,
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
					On("GetRevision").Return(int64(34))
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
				osm *ownerStrategyMock,
				owner *phaseObjectOwnerMock,
			) {
				osm.
					On("IsController", mock.Anything, mock.Anything).
					Return(false)
				ownerObj := &unstructured.Unstructured{
					Object: map[string]any{},
				}
				owner.On("ClientObject").Return(ownerObj)
				owner.On("GetRevision").Return(int64(1))
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
				osm *ownerStrategyMock,
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
					On("GetRevision").Return(int64(100))
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
				osm *ownerStrategyMock,
				owner *phaseObjectOwnerMock,
			) {
				ownerObj := &unstructured.Unstructured{}
				owner.On("ClientObject").Return(ownerObj)
				owner.
					On("GetRevision").Return(int64(100))

				osm.
					On("IsController", ownerObj, mock.Anything).
					Return(false)
				osm.
					On("HasController", mock.Anything).
					Return(false)
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
				osm *ownerStrategyMock,
				owner *phaseObjectOwnerMock,
			) {
				osm.
					On("IsController", mock.Anything, mock.Anything).
					Return(false)
				ownerObj := &unstructured.Unstructured{
					Object: map[string]interface{}{},
				}
				osm.
					On("HasController", mock.Anything).
					Return(true)
				owner.On("ClientObject").Return(ownerObj)
				owner.On("GetRevision").Return(int64(1))
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

			os := &ownerStrategyMock{}
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
}

func Test_defaultAdoptionChecker_isControlledByPreviousRevision(t *testing.T) {
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
			os := &ownerStrategyMock{}
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
			previous.On("GetRemotePhases").Return([]corev1alpha1.RemotePhaseReference{
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
}

func Test_defaultPatcher_patchObject_update_metadata(t *testing.T) {
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
}

func Test_defaultPatcher_patchObject_update_no_metadata(t *testing.T) {
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
}

func Test_defaultPatcher_fixFieldManagers(t *testing.T) {
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
}

func Test_defaultPatcher_fixFieldManagers_error(t *testing.T) {
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
}

// Test_defaultPatcher_fixFieldManagers_nowork ensures that fixFieldManagers
// does not call Patch when no changes are needed.
func Test_defaultPatcher_fixFieldManagers_nowork(t *testing.T) {
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
			owner.On("GetConditions").Return(&conditions)

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
	pr := &PhaseReconciler{
		scheme:           testScheme,
		preflightChecker: pcm,
	}

	ownerObj := &unstructured.Unstructured{}
	owner := &phaseObjectOwnerMock{}
	owner.On("ClientObject").Return(ownerObj)
	owner.On("GetRevision").Return(int64(12))

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

type preflightCheckerMock struct {
	mock.Mock
}

func (m *preflightCheckerMock) Check(
	ctx context.Context, owner, obj client.Object,
) (violations []preflight.Violation, err error) {
	args := m.Called(ctx, owner, obj)
	return args.Get(0).([]preflight.Violation), args.Error(1)
}

var errTest = errors.New("xxx")

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

type testUpdateMock struct {
	mock.Mock
}

func (m *testUpdateMock) Update(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type objectSetOrPhaseStub struct {
	ObjectSet corev1alpha1.ObjectSet
}

func (s *objectSetOrPhaseStub) ClientObject() client.Object {
	return &s.ObjectSet
}

func (s *objectSetOrPhaseStub) GetConditions() *[]metav1.Condition {
	return &s.ObjectSet.Status.Conditions
}
func (s *objectSetOrPhaseStub) UpdateStatusPhase() {}

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
		assert.Empty(t, objectSet.GetConditions())
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
		if assert.NotEmpty(t, objectSet.GetConditions()) {
			cond := meta.FindStatusCondition(*objectSet.GetConditions(), corev1alpha1.ObjectSetAvailable)
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
		if assert.NotEmpty(t, objectSet.GetConditions()) {
			cond := meta.FindStatusCondition(*objectSet.GetConditions(), corev1alpha1.ObjectSetAvailable)
			assert.Equal(t, "CollisionDetected", cond.Reason)
		}

		um.AssertExpectations(t)
	})
}
