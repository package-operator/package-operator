package testutil

import (
	"context"

	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CtrlClient is a mock for the controller-runtime client interface.
type CtrlClient struct {
	mock.Mock

	StatusMock *CtrlStatusClient
}

var _ client.Client = &CtrlClient{}

func NewClient() *CtrlClient {
	return &CtrlClient{
		StatusMock: &CtrlStatusClient{},
	}
}

func (c *CtrlClient) SubResource(string) client.SubResourceClient { panic("nip") }

// StatusClient interface

func (c *CtrlClient) Status() client.StatusWriter {
	return c.StatusMock
}

// Reader interface

func (c *CtrlClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	args := c.Called(ctx, key, obj, opts)
	return args.Error(0)
}

func (c *CtrlClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := c.Called(ctx, list, opts)
	return args.Error(0)
}

// Writer interface

func (c *CtrlClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}

func (c *CtrlClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}

func (c *CtrlClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}

func (c *CtrlClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	args := c.Called(ctx, obj, patch, opts)
	return args.Error(0)
}

func (c *CtrlClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}

func (c *CtrlClient) Scheme() *runtime.Scheme {
	args := c.Called()
	return args.Get(0).(*runtime.Scheme)
}

func (c *CtrlClient) RESTMapper() meta.RESTMapper {
	args := c.Called()
	return args.Get(0).(meta.RESTMapper)
}

type CtrlStatusClient struct {
	mock.Mock
}

var _ client.StatusWriter = &CtrlStatusClient{}

func (c *CtrlStatusClient) Update(
	ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption,
) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}

func (c *CtrlStatusClient) Patch(
	ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption,
) error {
	args := c.Called(ctx, obj, patch, opts)
	return args.Error(0)
}

func (c *CtrlStatusClient) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	args := c.Called(ctx, obj, subResource, opts)
	return args.Error(0)
}
