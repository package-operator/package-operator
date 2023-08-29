package bootstrap

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/testutil"
)

func TestIsPKOAvailable(t *testing.T) {
	t.Parallel()

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		c := testutil.NewClient()

		c.On("Get",
			mock.Anything,
			mock.Anything,
			mock.IsType(&appsv1.Deployment{}),
			mock.Anything).
			Return(errors.NewNotFound(schema.GroupResource{}, ""))

		isPKOAvailable, err := isPKOAvailable(
			context.Background(), c, "")
		require.NoError(t, err)
		assert.False(t, isPKOAvailable)
	})

	t.Run("not available", func(t *testing.T) {
		t.Parallel()
		c := testutil.NewClient()

		c.On("Get", mock.Anything, mock.Anything,
			mock.AnythingOfType("*v1.Deployment"),
			mock.Anything).
			Return(nil)

		isPKOAvailable, err := isPKOAvailable(
			context.Background(), c, "")
		require.NoError(t, err)
		assert.False(t, isPKOAvailable)
	})

	t.Run("available", func(t *testing.T) {
		t.Parallel()

		c := testutil.NewClient()

		c.On("Get", mock.Anything, mock.Anything,
			mock.AnythingOfType("*v1.Deployment"),
			mock.Anything).
			Run(func(args mock.Arguments) {
				depl := args.Get(2).(*appsv1.Deployment)
				depl.Status.AvailableReplicas = 1
				depl.Status.UpdatedReplicas = depl.Status.AvailableReplicas
			}).
			Return(nil)

		c.On("Get", mock.Anything, mock.Anything,
			mock.AnythingOfType("*v1alpha1.ClusterPackage"),
			mock.Anything).
			Run(func(args mock.Arguments) {
				cp := args.Get(2).(*corev1alpha1.ClusterPackage)
				cp.Generation = 5
				meta.SetStatusCondition(&cp.Status.Conditions, metav1.Condition{
					Type:               corev1alpha1.PackageAvailable,
					Status:             metav1.ConditionTrue,
					ObservedGeneration: cp.Generation,
				})
			}).
			Return(nil)

		isPKOAvailable, err := isPKOAvailable(
			context.Background(), c, "")
		require.NoError(t, err)
		assert.True(t, isPKOAvailable)
	})
}
