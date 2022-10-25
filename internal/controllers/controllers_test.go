package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
)

func TestEnsureFinalizer(t *testing.T) {
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
		assert.Equal(t, `{"metadata":{"finalizers":["already-present","test-finalizer"],"resourceVersion":"xxx-123"}}`, string(j))
	}
}

func TestRemoveFinalizer(t *testing.T) {
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
		assert.Equal(t, `{"metadata":{"finalizers":["already-present"],"resourceVersion":"xxx-123"}}`, string(j))
	}
}

func TestReportOwnActiveObjects(t *testing.T) {
	ctx := context.Background()
	ownerStrategy := &ownerStrategyMock{}
	ownerStrategy.
		On("IsController", mock.Anything, mock.AnythingOfType("*v1.Secret")).
		Return(true)
	ownerStrategy.
		On("IsController", mock.Anything, mock.Anything).
		Return(false)

	activeObjects, err := GetControllerOf(
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
