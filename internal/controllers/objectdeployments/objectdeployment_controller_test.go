package objectdeployments

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/testutil"
)

func TestObjectDeployment_Reconciler(t *testing.T) {
	t.Parallel()

	fooErr := errors.NewBadRequest("foo")

	clientMock := testutil.NewClient()
	c := NewObjectDeploymentController(
		clientMock, ctrl.Log.WithName("object deployment test"), testScheme)

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectDeployment"), mock.Anything).
		Return(fooErr)

	ctx := context.Background()
	res, err := c.Reconcile(ctx, ctrl.Request{})

	require.Error(t, err)
	assert.Empty(t, res)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
}

func TestObjectDeploymentReconciler_NotFound(t *testing.T) {
	t.Parallel()

	fooErr := errors.NewBadRequest("foo")

	clientMock := testutil.NewClient()
	c := NewObjectDeploymentController(
		clientMock, ctrl.Log.WithName("object deployment test"), testScheme)

	odName := "testing-123"
	od := &corev1alpha1.ObjectDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: odName,
		},
	}

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectDeployment"), mock.Anything).
		Return(fooErr)

	ctx := context.Background()
	res, err := c.Reconcile(ctx, ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(od),
	})

	require.Error(t, err)
	assert.Empty(t, res)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
}
