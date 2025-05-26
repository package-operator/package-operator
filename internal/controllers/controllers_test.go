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
