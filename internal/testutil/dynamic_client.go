package testutil

import (
	"context"

	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

var (
	_ dynamic.Interface                      = (*DynamicClient)(nil)
	_ dynamic.ResourceInterface              = (*DynamicClientResourceInterface)(nil)
	_ dynamic.NamespaceableResourceInterface = (*DynamicClientNamespaceableResourceInterface)(nil)
)

type DynamicClient struct {
	mock.Mock
}

func NewDynamicClient() *DynamicClient {
	return &DynamicClient{}
}

func (dc *DynamicClient) Resource(
	resource schema.GroupVersionResource,
) dynamic.NamespaceableResourceInterface {
	args := dc.Called(resource)
	return args.Get(0).(dynamic.NamespaceableResourceInterface)
}

type DynamicClientResourceInterface struct {
	mock.Mock
}

func (dc *DynamicClientResourceInterface) Create(
	ctx context.Context, obj *unstructured.Unstructured,
	options metav1.CreateOptions, subresources ...string,
) (*unstructured.Unstructured, error) {
	args := dc.Called(ctx, obj, options, subresources)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (dc *DynamicClientResourceInterface) Update(
	ctx context.Context, obj *unstructured.Unstructured,
	options metav1.UpdateOptions, subresources ...string,
) (*unstructured.Unstructured, error) {
	args := dc.Called(ctx, obj, options, subresources)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (dc *DynamicClientResourceInterface) UpdateStatus(
	ctx context.Context, obj *unstructured.Unstructured,
	options metav1.UpdateOptions,
) (*unstructured.Unstructured, error) {
	args := dc.Called(ctx, obj, options)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (dc *DynamicClientResourceInterface) Delete(
	ctx context.Context, name string,
	options metav1.DeleteOptions, subresources ...string,
) error {
	args := dc.Called(ctx, name, options, subresources)
	return args.Error(0)
}

func (dc *DynamicClientResourceInterface) DeleteCollection(
	ctx context.Context, options metav1.DeleteOptions,
	listOptions metav1.ListOptions,
) error {
	args := dc.Called(ctx, options, listOptions)
	return args.Error(0)
}

func (dc *DynamicClientResourceInterface) Get(
	ctx context.Context, name string,
	options metav1.GetOptions, subresources ...string,
) (*unstructured.Unstructured, error) {
	args := dc.Called(ctx, name, options, subresources)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (dc *DynamicClientResourceInterface) List(
	ctx context.Context, opts metav1.ListOptions,
) (*unstructured.UnstructuredList, error) {
	args := dc.Called(ctx, opts)
	return args.Get(0).(*unstructured.UnstructuredList), args.Error(1)
}

func (dc *DynamicClientResourceInterface) Watch(
	ctx context.Context, opts metav1.ListOptions,
) (watch.Interface, error) {
	args := dc.Called(ctx, opts)
	return args.Get(0).(watch.Interface), args.Error(1)
}

func (dc *DynamicClientResourceInterface) Patch(
	ctx context.Context, name string,
	pt types.PatchType, data []byte,
	options metav1.PatchOptions, subresources ...string,
) (*unstructured.Unstructured, error) {
	args := dc.Called(ctx, name, pt, data, options, subresources)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (dc *DynamicClientResourceInterface) Apply(
	ctx context.Context, name string,
	obj *unstructured.Unstructured,
	options metav1.ApplyOptions, subresources ...string,
) (*unstructured.Unstructured, error) {
	args := dc.Called(ctx, name, obj, options, subresources)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (dc *DynamicClientResourceInterface) ApplyStatus(
	ctx context.Context, name string,
	obj *unstructured.Unstructured,
	options metav1.ApplyOptions,
) (*unstructured.Unstructured, error) {
	args := dc.Called(ctx, name, obj, options)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

type DynamicClientNamespaceableResourceInterface struct {
	mock.Mock
}

func (dc *DynamicClientNamespaceableResourceInterface) Namespace(
	namespace string,
) dynamic.ResourceInterface {
	args := dc.Called(namespace)
	return args.Get(0).(dynamic.ResourceInterface)
}

func (dc *DynamicClientNamespaceableResourceInterface) Create(
	ctx context.Context, obj *unstructured.Unstructured,
	options metav1.CreateOptions, subresources ...string,
) (*unstructured.Unstructured, error) {
	args := dc.Called(ctx, obj, options, subresources)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (dc *DynamicClientNamespaceableResourceInterface) Update(
	ctx context.Context, obj *unstructured.Unstructured,
	options metav1.UpdateOptions, subresources ...string,
) (*unstructured.Unstructured, error) {
	args := dc.Called(ctx, obj, options, subresources)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (dc *DynamicClientNamespaceableResourceInterface) UpdateStatus(
	ctx context.Context, obj *unstructured.Unstructured,
	options metav1.UpdateOptions,
) (*unstructured.Unstructured, error) {
	args := dc.Called(ctx, obj, options)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (dc *DynamicClientNamespaceableResourceInterface) Delete(
	ctx context.Context, name string,
	options metav1.DeleteOptions, subresources ...string,
) error {
	args := dc.Called(ctx, name, options, subresources)
	return args.Error(0)
}

func (dc *DynamicClientNamespaceableResourceInterface) DeleteCollection(
	ctx context.Context, options metav1.DeleteOptions,
	listOptions metav1.ListOptions,
) error {
	args := dc.Called(ctx, options, listOptions)
	return args.Error(0)
}

func (dc *DynamicClientNamespaceableResourceInterface) Get(
	ctx context.Context, name string,
	options metav1.GetOptions, subresources ...string,
) (*unstructured.Unstructured, error) {
	args := dc.Called(ctx, name, options, subresources)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (dc *DynamicClientNamespaceableResourceInterface) List(
	ctx context.Context, opts metav1.ListOptions,
) (*unstructured.UnstructuredList, error) {
	args := dc.Called(ctx, opts)
	return args.Get(0).(*unstructured.UnstructuredList), args.Error(1)
}

func (dc *DynamicClientNamespaceableResourceInterface) Watch(
	ctx context.Context, opts metav1.ListOptions,
) (watch.Interface, error) {
	args := dc.Called(ctx, opts)
	return args.Get(0).(watch.Interface), args.Error(1)
}

func (dc *DynamicClientNamespaceableResourceInterface) Patch(
	ctx context.Context, name string,
	pt types.PatchType, data []byte,
	options metav1.PatchOptions, subresources ...string,
) (*unstructured.Unstructured, error) {
	args := dc.Called(ctx, name, pt, data, options, subresources)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (dc *DynamicClientNamespaceableResourceInterface) Apply(
	ctx context.Context, name string,
	obj *unstructured.Unstructured,
	options metav1.ApplyOptions, subresources ...string,
) (*unstructured.Unstructured, error) {
	args := dc.Called(ctx, name, obj, options, subresources)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (dc *DynamicClientNamespaceableResourceInterface) ApplyStatus(
	ctx context.Context, name string,
	obj *unstructured.Unstructured,
	options metav1.ApplyOptions,
) (*unstructured.Unstructured, error) {
	args := dc.Called(ctx, name, obj, options)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}
