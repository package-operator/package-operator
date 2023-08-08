package dynamiccachemocks

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"package-operator.run/internal/dynamiccache"
	"package-operator.run/internal/testutil"
)

type DynamicCacheMock struct {
	testutil.CtrlClient
}

func (c *DynamicCacheMock) Get(
	ctx context.Context,
	key client.ObjectKey, out client.Object,
	opts ...client.GetOption,
) error {
	args := c.Called(ctx, key, out, opts)
	return args.Error(0)
}

func (c *DynamicCacheMock) Source() source.Source {
	args := c.Called()
	return args.Get(0).(source.Source)
}

func (c *DynamicCacheMock) Free(ctx context.Context, obj client.Object) error {
	args := c.Called(ctx, obj)
	return args.Error(0)
}

func (c *DynamicCacheMock) Watch(
	ctx context.Context, owner client.Object, obj runtime.Object,
) error {
	args := c.Called(ctx, owner, obj)
	return args.Error(0)
}

func (c *DynamicCacheMock) OwnersForGKV(gvk schema.GroupVersionKind) []dynamiccache.OwnerReference {
	args := c.Called(gvk)
	return args.Get(0).([]dynamiccache.OwnerReference)
}
