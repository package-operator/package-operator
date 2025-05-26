package packages

import (
	"context"
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
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/metrics"
	"package-operator.run/internal/packages"
	"package-operator.run/internal/testutil"
)

var packageScheme = runtime.NewScheme()

func init() {
	if err := corev1alpha1.AddToScheme(packageScheme); err != nil {
		panic(err)
	}
}

func TestPackageController_Err(t *testing.T) {
	t.Parallel()

	fooErr := errors.NewBadRequest("foo")
	mr := metrics.NewRecorder()
	ipm := &imagePullerMock{}
	clientMock := testutil.NewClient()
	var hash int32
	c := NewPackageController(
		clientMock,
		clientMock,
		ctrl.Log.WithName("package test"),
		packageScheme,
		ipm,
		mr,
		&hash,
		nil,
	)

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
		Return(fooErr)

	ctx := context.Background()
	res, err := c.Reconcile(ctx, ctrl.Request{})

	require.Error(t, err)
	require.EqualError(t, err, fooErr.Error())
	assert.Empty(t, res)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
}

func TestPackageController_NotFound(t *testing.T) {
	t.Parallel()

	mr := metrics.NewRecorder()
	ipm := &imagePullerMock{}
	clientMock := testutil.NewClient()
	var hash int32
	c := NewPackageController(
		clientMock,
		clientMock,
		ctrl.Log.WithName("package test"),
		packageScheme,
		ipm,
		mr,
		&hash,
		nil,
	)
	c.reconciler = nil

	objectKey := client.ObjectKey{Name: "test", Namespace: "testns"}

	notFoundErr := errors.NewNotFound(schema.GroupResource{
		Group:    "package-operator.run",
		Resource: "Package",
	}, objectKey.Name)

	clientMock.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
		Return(notFoundErr)

	ctx := context.Background()
	res, err := c.Reconcile(ctx, reconcile.Request{
		NamespacedName: objectKey,
	})

	require.NoError(t, err)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
	clientMock.StatusMock.AssertNotCalled(
		t, "Update", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything,
	)
}

func TestPackageController_Paused(t *testing.T) {
	t.Parallel()

	mr := metrics.NewRecorder()
	ipm := &imagePullerMock{}
	clientMock := testutil.NewClient()
	var hash int32
	newPausedPackage := func(scheme *runtime.Scheme) adapters.PackageAccessor {
		obj := adapters.NewGenericPackage(scheme)
		obj.SetSpecPaused(true)
		return obj
	}

	c := newGenericPackageController(
		newPausedPackage,
		adapters.NewObjectDeployment,
		clientMock,
		clientMock,
		ctrl.Log.WithName("paused package test"),
		packageScheme,
		ipm,
		packages.NewClusterPackageDeployer(clientMock, packageScheme, nil),
		mr,
		&hash,
		nil,
	)
	c.reconciler = nil

	objectKey := client.ObjectKey{}

	clientMock.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
		Return(nil)
	clientMock.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.ObjectDeployment"), mock.Anything).
		Return(nil)
	clientMock.
		On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectDeployment"), mock.Anything).
		Return(nil)
	clientMock.
		On("Patch", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything, mock.Anything).
		Return(nil)
	clientMock.StatusMock.
		On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
		Return(nil)

	ctx := context.Background()
	res, err := c.Reconcile(ctx, reconcile.Request{
		NamespacedName: objectKey,
	})
	require.NoError(t, err)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
	clientMock.StatusMock.AssertExpectations(t)
}

func TestPackageController_Reconcile(t *testing.T) {
	t.Parallel()

	mr := metrics.NewRecorder()
	ipm := &imagePullerMock{}
	clientMock := testutil.NewClient()
	var hash int32
	c := NewPackageController(
		clientMock,
		clientMock,
		ctrl.Log.WithName("package test"),
		packageScheme,
		ipm,
		mr,
		&hash,
		nil,
	)
	c.reconciler = nil

	objectKey := client.ObjectKey{}

	clientMock.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
		Return(nil)
	clientMock.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.ObjectDeployment"), mock.Anything).
		Return(nil)
	clientMock.
		On("Patch", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything, mock.Anything).
		Return(nil)
	clientMock.StatusMock.
		On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
		Return(nil)

	ctx := context.Background()
	res, err := c.Reconcile(ctx, reconcile.Request{
		NamespacedName: objectKey,
	})
	require.NoError(t, err)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
	clientMock.StatusMock.AssertExpectations(t)
}

func TestClusterPackageController_Err(t *testing.T) {
	t.Parallel()

	fooErr := errors.NewBadRequest("foo")

	mr := metrics.NewRecorder()
	ipm := &imagePullerMock{}
	clientMock := testutil.NewClient()
	var hash int32
	c := NewClusterPackageController(
		clientMock,
		clientMock,
		ctrl.Log.WithName("clusterpackage test"),
		packageScheme,
		ipm,
		mr,
		&hash,
		nil,
	)
	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Return(fooErr)

	ctx := context.Background()
	res, err := c.Reconcile(ctx, ctrl.Request{})

	require.Error(t, err)
	require.EqualError(t, err, fooErr.Error())
	assert.Empty(t, res)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
}

func TestClusterOPackageController_NotFound(t *testing.T) {
	t.Parallel()

	mr := metrics.NewRecorder()
	ipm := &imagePullerMock{}
	clientMock := testutil.NewClient()
	var hash int32
	c := NewClusterPackageController(
		clientMock,
		clientMock,
		ctrl.Log.WithName("clusterpackage test"),
		packageScheme,
		ipm,
		mr,
		&hash,
		nil,
	)
	c.reconciler = nil

	objectKey := client.ObjectKey{Name: "test", Namespace: "testns"}

	notFoundErr := errors.NewNotFound(schema.GroupResource{
		Group:    "package-operator.run",
		Resource: "ClusterPackage",
	}, objectKey.Name)

	clientMock.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Return(notFoundErr)

	ctx := context.Background()
	res, err := c.Reconcile(ctx, reconcile.Request{
		NamespacedName: objectKey,
	})

	require.NoError(t, err)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
	clientMock.StatusMock.AssertNotCalled(
		t, "Update", mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything,
	)
}

func TestClusterPackageController_Paused(t *testing.T) {
	t.Parallel()

	mr := metrics.NewRecorder()
	ipm := &imagePullerMock{}
	clientMock := testutil.NewClient()
	var hash int32
	newPausedClusterPackage := func(scheme *runtime.Scheme) adapters.PackageAccessor {
		obj := adapters.NewGenericClusterPackage(scheme)
		obj.SetSpecPaused(true)
		return obj
	}

	c := newGenericPackageController(
		newPausedClusterPackage,
		adapters.NewClusterObjectDeployment,
		clientMock,
		clientMock,
		ctrl.Log.WithName("paused cluster package test"),
		packageScheme,
		ipm,
		packages.NewClusterPackageDeployer(clientMock, packageScheme, nil),
		mr,
		&hash,
		nil,
	)
	c.reconciler = nil

	objectKey := client.ObjectKey{}

	clientMock.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Return(nil)
	clientMock.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.ClusterObjectDeployment"), mock.Anything).
		Return(nil)
	clientMock.
		On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterObjectDeployment"), mock.Anything).
		Return(nil)
	clientMock.
		On("Patch", mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything, mock.Anything).
		Return(nil)
	clientMock.StatusMock.
		On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Return(nil)

	ctx := context.Background()
	res, err := c.Reconcile(ctx, reconcile.Request{
		NamespacedName: objectKey,
	})
	require.NoError(t, err)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
	clientMock.StatusMock.AssertExpectations(t)
}

func TestClusterPackageController_Reconcile(t *testing.T) {
	t.Parallel()

	mr := metrics.NewRecorder()
	ipm := &imagePullerMock{}
	clientMock := testutil.NewClient()
	var hash int32
	c := NewClusterPackageController(
		clientMock,
		clientMock,
		ctrl.Log.WithName("clusterpackage test"),
		packageScheme,
		ipm,
		mr,
		&hash,
		nil,
	)
	c.reconciler = nil

	objectKey := client.ObjectKey{Name: "", Namespace: ""}

	clientMock.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Return(nil)
	clientMock.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.ClusterObjectDeployment"), mock.Anything).
		Return(nil)
	clientMock.
		On("Patch", mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything, mock.Anything).
		Return(nil)
	clientMock.StatusMock.
		On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Return(nil)

	ctx := context.Background()
	res, err := c.Reconcile(ctx, reconcile.Request{
		NamespacedName: objectKey,
	})
	require.NoError(t, err)
	assert.True(t, res.IsZero())

	clientMock.AssertExpectations(t)
	clientMock.StatusMock.AssertExpectations(t)
}
