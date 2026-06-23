package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/testutil"
	"package-operator.run/internal/testutil/ownerhandlingmocks"
)

func TestEnsureFinalizer(t *testing.T) {
	t.Parallel()
	const finalizer = "test-finalizer"
	clientMock := testutil.NewClient()

	ctx := context.Background()
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "xxx-123",
			Finalizers: []string{
				"already-present",
			},
		},
	}

	var patch client.Patch
	clientMock.
		On("Patch", mock.Anything, obj, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			patch = args.Get(2).(client.Patch)
		}).
		Return(nil)

	err := EnsureFinalizer(ctx, clientMock, obj, finalizer)
	require.NoError(t, err)
	if assert.NotNil(t, patch) {
		j, err := patch.Data(obj)
		require.NoError(t, err)
		assert.JSONEq(t,
			`{"metadata":{"finalizers":["already-present","test-finalizer"],"resourceVersion":"xxx-123"}}`,
			string(j),
		)
	}
}

func TestRemoveFinalizer(t *testing.T) {
	t.Parallel()
	const finalizer = "test-finalizer"
	clientMock := testutil.NewClient()

	ctx := context.Background()
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "xxx-123",
			Finalizers: []string{
				"already-present",
				finalizer,
			},
		},
	}

	var patch client.Patch
	clientMock.
		On("Patch", mock.Anything, obj, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			patch = args.Get(2).(client.Patch)
		}).
		Return(nil)

	err := RemoveFinalizer(ctx, clientMock, obj, finalizer)
	require.NoError(t, err)
	if assert.NotNil(t, patch) {
		j, err := patch.Data(obj)
		require.NoError(t, err)
		assert.JSONEq(t, `{"metadata":{"finalizers":["already-present"],"resourceVersion":"xxx-123"}}`, string(j))
	}
}

func TestReportOwnActiveObjects(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	ownerStrategy.
		On("IsController", mock.Anything, mock.AnythingOfType("*v1.Secret")).
		Return(true)
	ownerStrategy.
		On("IsController", mock.Anything, mock.Anything).
		Return(false)

	activeObjects, err := GetStatusControllerOf(
		ctx, testScheme, ownerStrategy,
		&corev1.ConfigMap{},
		[]client.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: "ns-1",
				},
			},
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "ns-2",
				},
			},
		})
	require.NoError(t, err)

	assert.Equal(t, []corev1alpha1.ControlledObjectReference{
		{
			Kind:      "Secret",
			Group:     "", // core API group
			Name:      "secret-1",
			Namespace: "ns-1",
		},
	}, activeObjects)
}

func TestIsMappedCondition(t *testing.T) {
	t.Parallel()
	assert.False(t, IsMappedCondition(metav1.Condition{
		Type: "Available",
	}))

	assert.True(t, IsMappedCondition(metav1.Condition{
		Type: "my-prefix/Available",
	}))
}

func TestMapConditions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                   string
		srcGeneration          int64
		srcConditions          []metav1.Condition
		destGeneration         int64
		expectedDestConditions []metav1.Condition
	}{
		{
			name:          "mapping",
			srcGeneration: 4,
			srcConditions: []metav1.Condition{
				{
					Type:               "Available",
					Status:             metav1.ConditionTrue,
					Reason:             "MyReason",
					Message:            "message",
					ObservedGeneration: 4,
				},
				{
					Type:               "my-prefix/Available",
					Status:             metav1.ConditionTrue,
					Reason:             "MyReason",
					Message:            "message",
					ObservedGeneration: 4,
				},
				{
					Type:               "my-prefix/Banana",
					Status:             metav1.ConditionTrue,
					Reason:             "MyReason",
					Message:            "message",
					ObservedGeneration: 3,
				},
			},
			destGeneration: 42,
			expectedDestConditions: []metav1.Condition{
				{
					Type:               "my-prefix/Available",
					Status:             metav1.ConditionTrue,
					Reason:             "MyReason",
					Message:            "message",
					ObservedGeneration: 42,
				},
			},
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			var destConditions []metav1.Condition
			MapConditions(
				ctx, test.srcGeneration, test.srcConditions,
				test.destGeneration, &destConditions,
			)
			if assert.Len(t, destConditions, len(test.expectedDestConditions)) {
				for i := range test.expectedDestConditions {
					expected := test.expectedDestConditions[i]
					got := destConditions[i]

					assert.Equal(t, expected.Type, got.Type)
					assert.Equal(t, expected.Status, got.Status)
					assert.Equal(t, expected.Reason, got.Reason)
					assert.Equal(t, expected.Message, got.Message)
					assert.Equal(t, expected.ObservedGeneration, got.ObservedGeneration)
					assert.NotEmpty(t, got.LastTransitionTime)
				}
			}
		})
	}
}

func TestDeleteMappedConditions(t *testing.T) {
	t.Parallel()
	conditions := []metav1.Condition{
		{
			Type: "Available",
		},
		{
			Type: "test/Available",
		},
	}
	DeleteMappedConditions(context.Background(), &conditions)

	assert.Equal(t, []metav1.Condition{
		{Type: "Available"},
	}, conditions)
}

func TestAddDynamicCacheLabel(t *testing.T) {
	t.Parallel()

	expectedLabels := map[string]string{
		constants.DynamicCacheLabel: "True",
	}

	object := &unstructured.Unstructured{}

	c := testutil.NewClient()
	c.
		On("Patch",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).
		Return(nil)

	updated, err := AddDynamicCacheLabel(context.Background(), c, object)
	require.NoError(t, err)

	assert.Equal(t, expectedLabels, updated.GetLabels())
}

func TestRemoveDynamicCacheLabel(t *testing.T) {
	t.Parallel()

	expectedLabels := map[string]string{}

	object := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					constants.DynamicCacheLabel: "True",
				},
			},
		},
	}

	c := testutil.NewClient()
	c.
		On("Patch",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).
		Return(nil)

	updated, err := RemoveDynamicCacheLabel(context.Background(), c, object)
	require.NoError(t, err)

	assert.Equal(t, expectedLabels, updated.GetLabels())
}

func TestEnsureFinalizer_AlreadyPresent(t *testing.T) {
	t.Parallel()
	const finalizer = "test-finalizer"
	clientMock := testutil.NewClient()

	ctx := context.Background()
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "xxx-123",
			Finalizers: []string{
				finalizer,
			},
		},
	}

	// Should not call Patch when finalizer is already present
	err := EnsureFinalizer(ctx, clientMock, obj, finalizer)
	require.NoError(t, err)
	clientMock.AssertNotCalled(t, "Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestEnsureFinalizer_PatchError(t *testing.T) {
	t.Parallel()
	const finalizer = "test-finalizer"
	clientMock := testutil.NewClient()

	ctx := context.Background()
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "xxx-123",
		},
	}

	expectedErr := assert.AnError
	clientMock.
		On("Patch", mock.Anything, obj, mock.Anything, mock.Anything).
		Return(expectedErr)

	err := EnsureFinalizer(ctx, clientMock, obj, finalizer)
	require.Error(t, err)
	assert.ErrorContains(t, err, "adding finalizer")
}

func TestRemoveFinalizer_NotPresent(t *testing.T) {
	t.Parallel()
	const finalizer = "test-finalizer"
	clientMock := testutil.NewClient()

	ctx := context.Background()
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "xxx-123",
			Finalizers: []string{
				"other-finalizer",
			},
		},
	}

	// Should not call Patch when finalizer is not present
	err := RemoveFinalizer(ctx, clientMock, obj, finalizer)
	require.NoError(t, err)
	clientMock.AssertNotCalled(t, "Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRemoveFinalizer_PatchError(t *testing.T) {
	t.Parallel()
	const finalizer = "test-finalizer"
	clientMock := testutil.NewClient()

	ctx := context.Background()
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "xxx-123",
			Finalizers: []string{
				finalizer,
			},
		},
	}

	expectedErr := assert.AnError
	clientMock.
		On("Patch", mock.Anything, obj, mock.Anything, mock.Anything).
		Return(expectedErr)

	err := RemoveFinalizer(ctx, clientMock, obj, finalizer)
	require.Error(t, err)
	assert.ErrorContains(t, err, "removing finalizer")
}

func TestEnsureCachedFinalizer(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	ctx := context.Background()
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "xxx-123",
		},
	}

	clientMock.
		On("Patch", mock.Anything, obj, mock.Anything, mock.Anything).
		Return(nil)

	err := EnsureCachedFinalizer(ctx, clientMock, obj)
	require.NoError(t, err)

	assert.Contains(t, obj.GetFinalizers(), constants.CachedFinalizer)
}

func TestRemoveCacheFinalizer(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	ctx := context.Background()
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "xxx-123",
			Finalizers: []string{
				constants.CachedFinalizer,
			},
		},
	}

	clientMock.
		On("Patch", mock.Anything, obj, mock.Anything, mock.Anything).
		Return(nil)

	err := RemoveCacheFinalizer(ctx, clientMock, obj)
	require.NoError(t, err)

	assert.NotContains(t, obj.GetFinalizers(), constants.CachedFinalizer)
}

func TestGetStatusControllerOf_EmptyList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}

	controllerOf, err := GetStatusControllerOf(
		ctx, testScheme, ownerStrategy,
		&corev1.ConfigMap{},
		[]client.Object{})
	require.NoError(t, err)

	assert.Empty(t, controllerOf)
}

func TestGetStatusControllerOf_NoControllers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	ownerStrategy.
		On("IsController", mock.Anything, mock.Anything).
		Return(false)

	controllerOf, err := GetStatusControllerOf(
		ctx, testScheme, ownerStrategy,
		&corev1.ConfigMap{},
		[]client.Object{
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "ns-1",
				},
			},
		})
	require.NoError(t, err)

	assert.Empty(t, controllerOf)
}

func TestMapConditions_EmptySource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var destConditions []metav1.Condition

	MapConditions(ctx, 1, []metav1.Condition{}, 2, &destConditions)

	assert.Empty(t, destConditions)
}

func TestMapConditions_MultipleUpdates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	destConditions := []metav1.Condition{
		{
			Type:               "my-prefix/OldCondition",
			Status:             metav1.ConditionFalse,
			Reason:             "OldReason",
			Message:            "old message",
			ObservedGeneration: 1,
		},
	}

	srcConditions := []metav1.Condition{
		{
			Type:               "my-prefix/OldCondition",
			Status:             metav1.ConditionTrue,
			Reason:             "NewReason",
			Message:            "new message",
			ObservedGeneration: 5,
		},
	}

	MapConditions(ctx, 5, srcConditions, 10, &destConditions)

	require.Len(t, destConditions, 1)
	assert.Equal(t, "my-prefix/OldCondition", destConditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, destConditions[0].Status)
	assert.Equal(t, "NewReason", destConditions[0].Reason)
	assert.Equal(t, "new message", destConditions[0].Message)
	assert.Equal(t, int64(10), destConditions[0].ObservedGeneration)
}

func TestDeleteMappedConditions_NoMapped(t *testing.T) {
	t.Parallel()

	conditions := []metav1.Condition{
		{Type: "Available"},
		{Type: "Progressing"},
	}
	original := make([]metav1.Condition, len(conditions))
	copy(original, conditions)

	DeleteMappedConditions(context.Background(), &conditions)

	assert.Equal(t, original, conditions)
}

func TestDeleteMappedConditions_AllMapped(t *testing.T) {
	t.Parallel()

	conditions := []metav1.Condition{
		{Type: "prefix1/Available"},
		{Type: "prefix2/Progressing"},
	}

	DeleteMappedConditions(context.Background(), &conditions)

	assert.Empty(t, conditions)
}

func TestAddDynamicCacheLabel_WithExistingLabels(t *testing.T) {
	t.Parallel()

	object := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					"existing-label": "value",
				},
			},
		},
	}

	c := testutil.NewClient()
	c.
		On("Patch",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).
		Return(nil)

	updated, err := AddDynamicCacheLabel(context.Background(), c, object)
	require.NoError(t, err)

	assert.Equal(t, "True", updated.GetLabels()[constants.DynamicCacheLabel])
	assert.Equal(t, "value", updated.GetLabels()["existing-label"])
}

func TestAddDynamicCacheLabel_PatchError(t *testing.T) {
	t.Parallel()

	object := &unstructured.Unstructured{}

	c := testutil.NewClient()
	expectedErr := assert.AnError
	c.
		On("Patch",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).
		Return(expectedErr)

	_, err := AddDynamicCacheLabel(context.Background(), c, object)
	require.Error(t, err)
	assert.ErrorContains(t, err, "patching dynamic cache label")
}

func TestRemoveDynamicCacheLabel_PatchError(t *testing.T) {
	t.Parallel()

	object := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					constants.DynamicCacheLabel: "True",
				},
			},
		},
	}

	c := testutil.NewClient()
	expectedErr := assert.AnError
	c.
		On("Patch",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).
		Return(expectedErr)

	_, err := RemoveDynamicCacheLabel(context.Background(), c, object)
	require.Error(t, err)
	assert.ErrorContains(t, err, "patching object labels")
}

func TestRemoveDynamicCacheLabel_PreservesOtherLabels(t *testing.T) {
	t.Parallel()

	object := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					constants.DynamicCacheLabel: "True",
					"other-label":               "value",
				},
			},
		},
	}

	c := testutil.NewClient()
	c.
		On("Patch",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).
		Return(nil)

	updated, err := RemoveDynamicCacheLabel(context.Background(), c, object)
	require.NoError(t, err)

	labels := updated.GetLabels()
	assert.NotContains(t, labels, constants.DynamicCacheLabel)
	assert.Equal(t, "value", labels["other-label"])
}
