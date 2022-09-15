package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/apis/core/v1alpha1"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestPhaseReconciler_TeardownPhase(t *testing.T) {
	t.Run("already gone", func(t *testing.T) {
		dynamicCache := &dynamicCacheMock{}
		ownerStrategy := &ownerStrategyMock{}
		r := &PhaseReconciler{
			dynamicCache:  dynamicCache,
			ownerStrategy: ownerStrategy,
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
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(errors.NewNotFound(schema.GroupResource{}, ""))

		ctx := context.Background()
		done, err := r.TeardownPhase(ctx, owner, corev1alpha1.ObjectSetTemplatePhase{
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: runtime.RawExtension{},
				},
			},
		})
		require.NoError(t, err)
		assert.True(t, done)
		dynamicCache.AssertCalled(t, "Watch", mock.Anything, ownerObj, mock.Anything)
	})

	t.Run("already gone on delete", func(t *testing.T) {
		testClient := testutil.NewClient()
		dynamicCache := &dynamicCacheMock{}
		ownerStrategy := &ownerStrategyMock{}
		r := &PhaseReconciler{
			writer:        testClient,
			dynamicCache:  dynamicCache,
			ownerStrategy: ownerStrategy,
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
		currentObj := &unstructured.Unstructured{}
		dynamicCache.
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
			Return(errors.NewNotFound(schema.GroupResource{}, ""))

		ctx := context.Background()
		done, err := r.TeardownPhase(ctx, owner, corev1alpha1.ObjectSetTemplatePhase{
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: runtime.RawExtension{},
				},
			},
		})
		require.NoError(t, err)
		assert.True(t, done)
		dynamicCache.AssertCalled(t, "Watch", mock.Anything, ownerObj, mock.Anything)

		// Ensure that IsController was called with currentObj and not desiredObj.
		// If checking desiredObj, IsController will _always_ return true, which could lead to really nasty behavior.
		ownerStrategy.AssertCalled(t, "IsController", ownerObj, currentObj)
	})

	t.Run("delete waits", func(t *testing.T) {
		// delete returns false first,
		// we are only really done when the object is gone
		// from the apiserver after all finalizers are handled.
		testClient := testutil.NewClient()
		dynamicCache := &dynamicCacheMock{}
		ownerStrategy := &ownerStrategyMock{}
		r := &PhaseReconciler{
			writer:        testClient,
			dynamicCache:  dynamicCache,
			ownerStrategy: ownerStrategy,
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
		currentObj := &unstructured.Unstructured{}
		dynamicCache.
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
					Object: runtime.RawExtension{},
				},
			},
		})
		require.NoError(t, err)
		assert.False(t, done) // wait for delete confirm
		dynamicCache.AssertCalled(t, "Watch", mock.Anything, ownerObj, mock.Anything)

		// It's super important that we don't check ownership on desiredObj on accident, because that will always return true.
		ownerStrategy.AssertCalled(t, "IsController", ownerObj, currentObj)
	})

	t.Run("not controller", func(t *testing.T) {
		dynamicCache := &dynamicCacheMock{}
		ownerStrategy := &ownerStrategyMock{}
		testClient := testutil.NewClient()
		r := &PhaseReconciler{
			dynamicCache:  dynamicCache,
			ownerStrategy: ownerStrategy,
			writer:        testClient,
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
		currentObj := &unstructured.Unstructured{}
		dynamicCache.
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				out := args.Get(2).(*unstructured.Unstructured)
				*out = *currentObj
			}).
			Return(nil)

		ownerStrategy.
			On("IsController", ownerObj, currentObj).
			Return(false)
		ownerStrategy.
			On("RemoveOwner", ownerObj, currentObj).
			Return(false)

		testClient.
			On("Update", mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		ctx := context.Background()
		done, err := r.TeardownPhase(ctx, owner, corev1alpha1.ObjectSetTemplatePhase{
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: runtime.RawExtension{},
				},
			},
		})
		require.NoError(t, err)
		assert.True(t, done)
		dynamicCache.AssertCalled(t, "Watch", mock.Anything, ownerObj, mock.Anything)

		// It's super important that we don't check ownership on desiredObj on accident, because that will always return true.
		ownerStrategy.AssertCalled(t, "IsController", ownerObj, currentObj)
		ownerStrategy.AssertCalled(t, "RemoveOwner", ownerObj, currentObj)
		testClient.AssertCalled(t, "Update", mock.Anything, currentObj, mock.Anything)
	})
}

func TestPhaseReconciler_reconcileObject_create(t *testing.T) {
	testClient := testutil.NewClient()
	dynamicCacheMock := &dynamicCacheMock{}
	r := &PhaseReconciler{
		writer:       testClient,
		dynamicCache: dynamicCacheMock,
	}
	owner := &phaseObjectOwnerMock{}

	dynamicCacheMock.
		On("Get", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.NewNotFound(schema.GroupResource{}, ""))
	testClient.
		On("Create", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	ctx := context.Background()
	desired := &unstructured.Unstructured{}
	actual, err := r.reconcileObject(ctx, owner, desired, nil)
	require.NoError(t, err)

	assert.Same(t, desired, actual)
}

func TestPhaseReconciler_reconcileObject_update(t *testing.T) {
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
		On("Check", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)

	dynamicCacheMock.
		On("Get", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	ownerStrategy.On("ReleaseController", mock.Anything)
	ownerStrategy.
		On("SetControllerReference", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	ownerStrategy.
		On("IsController", mock.Anything, mock.Anything).
		Return(true)

	patcher.
		On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	ctx := context.Background()
	actual, err := r.reconcileObject(ctx, owner, &unstructured.Unstructured{}, nil)
	require.NoError(t, err)

	assert.Equal(t, &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					revisionAnnotation: "3",
				},
			},
		},
	}, actual)
}

func TestPhaseReconciler_desiredObject(t *testing.T) {
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
	owner.On("ClientObject").Return(ownerObj)
	owner.On("GetRevision").Return(int64(5))

	phaseObject := corev1alpha1.ObjectSetObject{
		Object: runtime.RawExtension{
			Raw: []byte(`{"kind": "test"}`),
		},
	}
	desiredObj, err := r.desiredObject(ctx, owner, phaseObject)
	require.NoError(t, err)

	assert.Equal(t, &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind": "test",
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					revisionAnnotation: "5",
				},
				"labels": map[string]interface{}{
					DynamicCacheLabel: "True",
				},
			},
		},
	}, desiredObj)
}

func Test_defaultAdoptionChecker_Check(t *testing.T) {
	tests := []struct {
		name          string
		mockPrepare   func(*ownerStrategyMock, *phaseObjectOwnerMock)
		object        client.Object
		previous      []PreviousObjectSet
		errorAs       interface{}
		needsAdoption bool
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
					Object: map[string]interface{}{},
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
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							revisionAnnotation: "15",
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
					Object: map[string]interface{}{},
				}
				owner.On("ClientObject").Return(ownerObj)
				osm.
					On("IsController", ownerObj, mock.Anything).
					Return(true)
			},
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{},
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
					Object: map[string]interface{}{},
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
					&unstructured.Unstructured{})},
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							revisionAnnotation: "100",
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
					Object: map[string]interface{}{},
				}
				owner.On("ClientObject").Return(ownerObj)
				owner.On("GetRevision").Return(int64(1))
			},
			previous: []PreviousObjectSet{
				newPreviousObjectSetMockWithoutRemotes(
					&unstructured.Unstructured{})},
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{},
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
					&corev1.ConfigMap{})},
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							revisionAnnotation: "100",
						},
					},
				},
			},
			errorAs:       &RevisionCollisionError{},
			needsAdoption: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			os := &ownerStrategyMock{}
			c := &defaultAdoptionChecker{
				ownerStrategy: os,
				scheme:        testScheme,
			}
			owner := &phaseObjectOwnerMock{}

			test.mockPrepare(os, owner)

			ctx := context.Background()
			needsAdoption, err := c.Check(
				ctx, owner, test.object, test.previous)
			if test.errorAs == nil {
				require.NoError(t, err)
			} else {
				require.ErrorAs(t, err, test.errorAs)
			}
			assert.Equal(t, test.needsAdoption, needsAdoption)
		})
	}
}

func Test_defaultAdoptionChecker_isControlledByPreviousRevision(t *testing.T) {
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
		mock.AnythingOfType("*unstructured.Unstructured"),
		mock.Anything,
	).Return(true)

	previousObj := &corev1alpha1.ObjectSet{}
	previous := &previousObjectSetMock{}
	previous.On("ClientObject").Return(previousObj)
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
					Controller: pointer.BoolPtr(true),
				},
			},
		},
	}

	isController := ac.isControlledByPreviousRevision(
		obj, []PreviousObjectSet{previous})
	assert.True(t, isController)
}

func Test_defaultPatcher_patchObject_update_metadata(t *testing.T) {
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
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"my-cool-label": "hans",
				},
			},
		},
	}
	currentObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"resourceVersion": "123",
				"labels": map[string]interface{}{
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

		assert.Equal(t,
			`{"metadata":{"labels":{"banana":"hans","my-cool-label":"hans"},"resourceVersion":"123"}}`, string(patch))
	}
}

func Test_defaultPatcher_patchObject_update_no_metadata(t *testing.T) {
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
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"my-cool-label": "hans",
				},
			},
			"spec": map[string]interface{}{
				"key": "val",
			},
		},
	}
	currentObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"resourceVersion": "123",
				"labels": map[string]interface{}{
					"banana":        "hans", // we don't care about extra labels
					"my-cool-label": "hans",
				},
			},
			"spec": map[string]interface{}{
				"key": "something else",
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

		assert.Equal(t,
			`{"spec":{"key":"val"}}`, string(patch))
	}
}

func Test_defaultPatcher_patchObject_noop(t *testing.T) {
	clientMock := testutil.NewClient()
	r := &defaultPatcher{
		writer: clientMock,
	}
	ctx := context.Background()

	clientMock.
		On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	// no need to patch anything, all objects are the same
	desiredObj := &unstructured.Unstructured{}
	currentObj := &unstructured.Unstructured{}
	updatedObj := &unstructured.Unstructured{}

	err := r.Patch(ctx, desiredObj, currentObj, updatedObj)
	require.NoError(t, err)

	clientMock.AssertNotCalled(
		t, "Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func Test_unstructuredFromObjectSetObject(t *testing.T) {
	u, err := unstructuredFromObjectSetObject(
		&v1alpha1.ObjectSetObject{
			Object: runtime.RawExtension{
				Raw: []byte(`{"kind":"test","metadata":{"name":"test"}}`),
			},
		})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"kind": "test",
		"metadata": map[string]interface{}{
			"name": "test",
		},
	}, u.Object)
}

func Test_mergeKeysFrom(t *testing.T) {
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := mergeKeysFrom(test.base, test.additional)
			assert.Equal(t, test.expected, r)
		})
	}
}
