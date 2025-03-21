package objectdeployments

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/testutil"
)

var deploymentTestScheme = runtime.NewScheme()

func init() {
	if err := corev1alpha1.AddToScheme(deploymentTestScheme); err != nil {
		panic(err)
	}
}

func TestObjectDeploymentController_Err(t *testing.T) {
	t.Parallel()

	fooErr := errors.NewBadRequest("foo")

	clientMock := testutil.NewClient()
	c := NewObjectDeploymentController(
		clientMock, ctrl.Log.WithName("object deployment test"), deploymentTestScheme)

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectDeployment"), mock.Anything).
		Return(fooErr)

	ctx := t.Context()
	res, err := c.Reconcile(ctx, ctrl.Request{})

	require.Error(t, err)
	require.EqualError(t, err, fooErr.Error())
	assert.Empty(t, res)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
}

func TestObjectDeploymentController_NotFound(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := NewObjectDeploymentController(
		clientMock, ctrl.Log.WithName("object deployment test"), deploymentTestScheme)
	c.reconciler = nil

	objectKey := client.ObjectKey{Name: "test", Namespace: "testns"}

	notFoundErr := errors.NewNotFound(schema.GroupResource{
		Group:    "package-operator.run",
		Resource: "ObjectDeployment",
	}, objectKey.Name)

	clientMock.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.ObjectDeployment"), mock.Anything).
		Return(notFoundErr)

	ctx := t.Context()
	res, err := c.Reconcile(ctx, reconcile.Request{
		NamespacedName: objectKey,
	})

	require.NoError(t, err)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
	clientMock.StatusMock.AssertNotCalled(
		t, "Update", mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectDeployment"), mock.Anything,
	)
}

func TestObjectDeploymentController_Reconcile(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := NewObjectDeploymentController(
		clientMock, ctrl.Log.WithName("object deployment test"), deploymentTestScheme)
	c.reconciler = nil

	objectKey := client.ObjectKey{Name: "test", Namespace: "testns"}

	clientMock.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.ObjectDeployment"), mock.Anything).
		Return(nil)
	clientMock.StatusMock.
		On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectDeployment"), mock.Anything).
		Return(nil)

	ctx := t.Context()
	res, err := c.Reconcile(ctx, reconcile.Request{
		NamespacedName: objectKey,
	})
	require.NoError(t, err)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
	clientMock.StatusMock.AssertExpectations(t)
}

func TestClusterObjectDeploymentController_Err(t *testing.T) {
	t.Parallel()

	fooErr := errors.NewBadRequest("foo")

	clientMock := testutil.NewClient()
	c := NewClusterObjectDeploymentController(
		clientMock, ctrl.Log.WithName("cluster object deployment test"), deploymentTestScheme)

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterObjectDeployment"), mock.Anything).
		Return(fooErr)

	ctx := t.Context()
	res, err := c.Reconcile(ctx, ctrl.Request{})

	require.Error(t, err)
	require.EqualError(t, err, fooErr.Error())
	assert.Empty(t, res)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
}

func TestClusterObjectDeploymentController_NotFound(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := NewClusterObjectDeploymentController(
		clientMock, ctrl.Log.WithName("cluster object deployment test"), deploymentTestScheme)
	c.reconciler = nil

	objectKey := client.ObjectKey{Name: "test", Namespace: "testns"}

	notFoundErr := errors.NewNotFound(schema.GroupResource{
		Group:    "package-operator.run",
		Resource: "ClusterObjectDeployment",
	}, objectKey.Name)

	clientMock.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.ClusterObjectDeployment"), mock.Anything).
		Return(notFoundErr)

	ctx := t.Context()
	res, err := c.Reconcile(ctx, reconcile.Request{
		NamespacedName: objectKey,
	})

	require.NoError(t, err)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
	clientMock.StatusMock.AssertNotCalled(
		t, "Update", mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterObjectDeployment"), mock.Anything,
	)
}

func TestClusterObjectDeploymentController_Reconcile(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := NewClusterObjectDeploymentController(
		clientMock, ctrl.Log.WithName("cluster object deployment test"), deploymentTestScheme)
	c.reconciler = nil

	objectKey := client.ObjectKey{Name: "test", Namespace: "testns"}

	clientMock.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.ClusterObjectDeployment"), mock.Anything).
		Return(nil)
	clientMock.StatusMock.
		On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterObjectDeployment"), mock.Anything).
		Return(nil)

	ctx := t.Context()
	res, err := c.Reconcile(ctx, reconcile.Request{
		NamespacedName: objectKey,
	})
	require.NoError(t, err)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
	clientMock.StatusMock.AssertExpectations(t)
}
