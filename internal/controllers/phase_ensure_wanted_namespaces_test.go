package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/addon-operator/internal/testutil"
)

func TestEnsureWantedNamespaces_AddonWithoutNamespaces(t *testing.T) {
	c := testutil.NewClient()

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: newTestSchemeWithAddonsv1alpha1(),
	}

	ctx := context.Background()
	stop, err := r.ensureWantedNamespaces(ctx, newTestAddonWithoutNamespace())
	require.NoError(t, err)
	require.False(t, stop)
	c.AssertExpectations(t)
}

func TestEnsureWantedNamespaces_AddonWithSingleNamespace_Collision(t *testing.T) {
	c := testutil.NewClient()
	c.On("Get", testutil.IsContext, testutil.IsObjectKey, testutil.IsCoreV1NamespacePtr).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		newTestExistingNamespaceWithOwner().DeepCopyInto(arg)
	}).Return(nil)
	c.StatusMock.On("Update", testutil.IsContext, testutil.IsAddonsv1alpha1AddonPtr, mock.Anything).Return(nil)

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: newTestSchemeWithAddonsv1alpha1(),
	}

	ctx := context.Background()
	stop, err := r.ensureWantedNamespaces(ctx, newTestAddonWithSingleNamespace())
	require.NoError(t, err)
	require.True(t, stop)
	c.AssertExpectations(t)
	c.AssertCalled(t, "Get", testutil.IsContext, testutil.IsObjectKey, testutil.IsCoreV1NamespacePtr)
	c.StatusMock.AssertCalled(t, "Update", testutil.IsContext, testutil.IsAddonsv1alpha1AddonPtr, mock.Anything)
}

func TestEnsureWantedNamespaces_AddonWithSingleNamespace_NoCollision(t *testing.T) {
	c := testutil.NewClient()
	c.On("Get", testutil.IsContext, testutil.IsObjectKey, testutil.IsCoreV1NamespacePtr).Return(newTestErrNotFound())
	c.On("Create", testutil.IsContext, testutil.IsCoreV1NamespacePtr, mock.Anything).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*corev1.Namespace)
		arg.Status = corev1.NamespaceStatus{
			Phase: corev1.NamespaceActive,
		}
	}).Return(nil)

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: newTestSchemeWithAddonsv1alpha1(),
	}

	ctx := context.Background()
	stop, err := r.ensureWantedNamespaces(ctx, newTestAddonWithSingleNamespace())
	require.NoError(t, err)
	require.False(t, stop)
	c.AssertExpectations(t)
	c.AssertCalled(t, "Get", testutil.IsContext, testutil.IsObjectKey, testutil.IsCoreV1NamespacePtr)
	c.AssertCalled(t, "Create", testutil.IsContext, testutil.IsCoreV1NamespacePtr, mock.Anything)
}

func TestEnsureWantedNamespaces_AddonWithMultipleNamespaces_NoCollision(t *testing.T) {
	c := testutil.NewClient()
	c.On("Get", testutil.IsContext, testutil.IsObjectKey, testutil.IsCoreV1NamespacePtr).Return(newTestErrNotFound())
	c.On("Create", testutil.IsContext, testutil.IsCoreV1NamespacePtr, mock.Anything).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*corev1.Namespace)
		arg.Status = corev1.NamespaceStatus{
			Phase: corev1.NamespaceActive,
		}
	}).Return(nil)

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: newTestSchemeWithAddonsv1alpha1(),
	}

	ctx := context.Background()
	stop, err := r.ensureWantedNamespaces(ctx, newTestAddonWithMultipleNamespaces())
	require.NoError(t, err)
	require.False(t, stop)
	// every namespace should have been created
	namespaceCount := len(newTestAddonWithMultipleNamespaces().Spec.Namespaces)
	c.AssertExpectations(t)
	c.AssertNumberOfCalls(t, "Get", namespaceCount)
	c.AssertNumberOfCalls(t, "Create", namespaceCount)
}

func TestEnsureWantedNamespaces_AddonWithMultipleNamespaces_SingleCollision(t *testing.T) {
	c := testutil.NewClient()
	c.On("Get", testutil.IsContext, testutil.IsObjectKey, testutil.IsCoreV1NamespacePtr).
		Return(newTestErrNotFound()).
		Once()
	c.On("Create", testutil.IsContext, testutil.IsCoreV1NamespacePtr, mock.Anything).
		Run(func(args mock.Arguments) {
			arg := args.Get(1).(*corev1.Namespace)
			arg.Status = corev1.NamespaceStatus{
				Phase: corev1.NamespaceActive,
			}
		}).
		Return(nil).
		Once()
	c.On("Get", testutil.IsContext, testutil.IsObjectKey, testutil.IsCoreV1NamespacePtr).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*corev1.Namespace)
			newTestExistingNamespaceWithOwner().DeepCopyInto(arg)
		}).
		Return(nil)
	c.StatusMock.On("Update", testutil.IsContext, testutil.IsAddonsv1alpha1AddonPtr, mock.Anything).
		Return(nil)

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: newTestSchemeWithAddonsv1alpha1(),
	}

	ctx := context.Background()
	stop, err := r.ensureWantedNamespaces(ctx, newTestAddonWithMultipleNamespaces())
	require.NoError(t, err)
	require.True(t, stop)
	c.AssertExpectations(t)
	c.AssertNumberOfCalls(t, "Get", len(newTestAddonWithMultipleNamespaces().Spec.Namespaces))
	c.StatusMock.AssertCalled(t, "Update", testutil.IsContext, testutil.IsAddonsv1alpha1AddonPtr, mock.Anything)

}
func TestEnsureWantedNamespaces_AddonWithMultipleNamespaces_MultipleCollisions(t *testing.T) {
	c := testutil.NewClient()
	c.On("Get", testutil.IsContext, testutil.IsObjectKey, testutil.IsCoreV1NamespacePtr).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*corev1.Namespace)
			newTestExistingNamespaceWithOwner().DeepCopyInto(arg)
		}).
		Return(nil)
	c.StatusMock.On("Update", testutil.IsContext, testutil.IsAddonsv1alpha1AddonPtr, mock.Anything).
		Return(nil)

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: newTestSchemeWithAddonsv1alpha1(),
	}

	ctx := context.Background()
	stop, err := r.ensureWantedNamespaces(ctx, newTestAddonWithMultipleNamespaces())
	require.NoError(t, err)
	require.True(t, stop)
	c.AssertExpectations(t)
	c.AssertNumberOfCalls(t, "Get", len(newTestAddonWithMultipleNamespaces().Spec.Namespaces))
	c.StatusMock.AssertCalled(t, "Update", testutil.IsContext, testutil.IsAddonsv1alpha1AddonPtr, mock.Anything)
}

func TestEnsureNamespace_Create(t *testing.T) {
	addon := newTestAddonWithSingleNamespace()

	c := testutil.NewClient()
	c.On("Get", testutil.IsContext, testutil.IsObjectKey, testutil.IsCoreV1NamespacePtr).Return(newTestErrNotFound())
	c.On("Create", testutil.IsContext, testutil.IsCoreV1NamespacePtr, mock.Anything).Return(nil)

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: newTestSchemeWithAddonsv1alpha1(),
	}

	ctx := context.Background()
	ensuredNamespace, err := r.ensureNamespace(ctx, addon, addon.Spec.Namespaces[0].Name)
	c.AssertExpectations(t)
	require.NoError(t, err)
	require.NotNil(t, ensuredNamespace)
}

func TestReconcileNamespace_Create(t *testing.T) {
	c := testutil.NewClient()
	c.On("Get", testutil.IsContext, testutil.IsObjectKey, testutil.IsCoreV1NamespacePtr).Return(newTestErrNotFound())
	c.On("Create", testutil.IsContext, testutil.IsCoreV1NamespacePtr, mock.Anything).Return(nil, newTestNamespace())

	ctx := context.Background()
	reconciledNamespace, err := reconcileNamespace(ctx, c, newTestNamespace())
	require.NoError(t, err)
	assert.NotNil(t, reconciledNamespace)
	assert.Equal(t, newTestNamespace(), reconciledNamespace)
	c.AssertExpectations(t)
	c.AssertCalled(t, "Get", testutil.IsContext, client.ObjectKey{
		Name: "namespace-1",
	}, testutil.IsCoreV1NamespacePtr)
	c.AssertCalled(t, "Create", testutil.IsContext, newTestNamespace(), mock.Anything)
}

func TestReconcileNamespace_CreateWithCollisionWithoutOwner(t *testing.T) {
	c := testutil.NewClient()
	c.On("Get", testutil.IsContext, testutil.IsObjectKey, testutil.IsCoreV1NamespacePtr).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		newTestExistingNamespaceWithoutOwner().DeepCopyInto(arg)
	}).Return(nil)

	ctx := context.Background()
	_, err := reconcileNamespace(ctx, c, newTestNamespace())
	require.EqualError(t, err, errNotOwnedByUs.Error())
	c.AssertExpectations(t)
	c.AssertCalled(t, "Get", testutil.IsContext, client.ObjectKey{
		Name: "namespace-1",
	}, testutil.IsCoreV1NamespacePtr)
}

func TestReconcileNamespace_CreateWithCollisionWithOtherOwner(t *testing.T) {
	c := testutil.NewClient()
	c.On("Get", testutil.IsContext, testutil.IsObjectKey, testutil.IsCoreV1NamespacePtr).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		newTestExistingNamespaceWithoutOwner().DeepCopyInto(arg)
	}).Return(nil)

	ctx := context.Background()
	_, err := reconcileNamespace(ctx, c, newTestNamespace())
	require.EqualError(t, err, errNotOwnedByUs.Error())
	c.AssertExpectations(t)
	c.AssertCalled(t, "Get", testutil.IsContext, client.ObjectKey{
		Name: "namespace-1",
	}, testutil.IsCoreV1NamespacePtr)
}

func TestReconcileNamespace_Update(t *testing.T) {
	c := testutil.NewClient()
	c.On("Get", testutil.IsContext, testutil.IsObjectKey, testutil.IsCoreV1NamespacePtr).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		newTestNamespace().DeepCopyInto(arg)
	}).Return(nil)
	c.On("Update", testutil.IsContext, testutil.IsCoreV1NamespacePtr, mock.Anything).Return(nil)

	ctx := context.Background()
	reconciledNamespace, err := reconcileNamespace(ctx, c, newTestNamespace())
	require.NoError(t, err)
	assert.NotNil(t, reconciledNamespace)
	assert.Equal(t, newTestNamespace(), reconciledNamespace)
	c.AssertExpectations(t)
	c.AssertCalled(t, "Get", testutil.IsContext, client.ObjectKey{
		Name: "namespace-1",
	}, testutil.IsCoreV1NamespacePtr)
	c.AssertCalled(t, "Update", testutil.IsContext, testutil.IsCoreV1NamespacePtr, mock.Anything)
}

func TestReconcileNamespace_CreateWithClientError(t *testing.T) {
	timeoutErr := k8sApiErrors.NewTimeoutError("for testing", 1)

	c := testutil.NewClient()
	c.On("Get", testutil.IsContext, testutil.IsObjectKey, testutil.IsCoreV1NamespacePtr).
		Return(timeoutErr)

	ctx := context.Background()
	_, err := reconcileNamespace(ctx, c, newTestNamespace())
	require.Error(t, err)
	require.EqualError(t, err, timeoutErr.Error())
	c.AssertExpectations(t)
	c.AssertCalled(t, "Get", testutil.IsContext, client.ObjectKey{
		Name: "namespace-1",
	}, testutil.IsCoreV1NamespacePtr)
}

func TestHasEqualControllerReference(t *testing.T) {
	require.True(t, hasEqualControllerReference(
		newTestNamespace(),
		newTestNamespace(),
	))

	require.False(t, hasEqualControllerReference(
		newTestNamespace(),
		newTestExistingNamespaceWithOwner(),
	))

	require.False(t, hasEqualControllerReference(
		newTestNamespace(),
		newTestExistingNamespaceWithoutOwner(),
	))
}
