package managedcachemocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"pkg.package-operator.run/boxcutter/managedcache"
)

var _ managedcache.Accessor = (*AccessorMock)(nil)

type AccessorMock struct {
	mock.Mock
}

func (m *AccessorMock) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *AccessorMock) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *AccessorMock) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *AccessorMock) Patch(
	ctx context.Context,
	obj client.Object,
	patch client.Patch,
	opts ...client.PatchOption,
) error {
	args := m.Called(ctx, obj, patch, opts)
	return args.Error(0)
}

func (m *AccessorMock) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *AccessorMock) Get(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...client.GetOption,
) error {
	args := m.Called(ctx, key, obj, opts)
	return args.Error(0)
}

func (m *AccessorMock) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := m.Called(ctx, list, opts)
	return args.Error(0)
}

func (m *AccessorMock) Source(handler handler.EventHandler, predicates ...predicate.Predicate) source.Source {
	args := m.Called(handler, predicates)
	return args.Get(0).(source.Source)
}

func (m *AccessorMock) RemoveOtherInformers(ctx context.Context, gvks sets.Set[schema.GroupVersionKind]) error {
	args := m.Called(ctx, gvks)
	return args.Error(0)
}

func (m *AccessorMock) GetGVKs() []schema.GroupVersionKind {
	return m.Called().Get(0).([]schema.GroupVersionKind)
}

func (m *AccessorMock) Free(ctx context.Context, user client.Object) error {
	args := m.Called()
	return args.Error(0)
}

func (m *AccessorMock) Watch(ctx context.Context, user client.Object, gvks sets.Set[schema.GroupVersionKind]) error {
	args := m.Called()
	return args.Error(0)
}

func (m *AccessorMock) GetInformer(
	ctx context.Context,
	obj client.Object,
	opts ...cache.InformerGetOption,
) (cache.Informer, error) {
	args := m.Called(ctx, obj, opts)
	return args.Get(0).(cache.Informer), args.Error(1)
}

func (m *AccessorMock) GetInformerForKind(
	ctx context.Context,
	gvk schema.GroupVersionKind,
	opts ...cache.InformerGetOption,
) (cache.Informer, error) {
	args := m.Called(ctx, gvk, opts)
	return args.Get(0).(cache.Informer), args.Error(1)
}

func (m *AccessorMock) RemoveInformer(ctx context.Context, obj client.Object) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

func (m *AccessorMock) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *AccessorMock) WaitForCacheSync(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

func (m *AccessorMock) IndexField(
	ctx context.Context,
	obj client.Object,
	field string,
	extractValue client.IndexerFunc,
) error {
	args := m.Called(ctx, obj, field, extractValue)
	return args.Error(0)
}

func (m *AccessorMock) Watch(ctx context.Context, user client.Object, gvks sets.Set[schema.GroupVersionKind]) error {
	args := m.Called(ctx, user, gvks)
	return args.Error(0)
}

func (m *AccessorMock) Free(ctx context.Context, user client.Object) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *AccessorMock) GetObjectsPerInformer(ctx context.Context) (map[schema.GroupVersionKind]int, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[schema.GroupVersionKind]int), args.Error(1)
}

func (m *AccessorMock) Apply(context.Context, runtime.ApplyConfiguration, ...client.ApplyOption) error {
	args := m.Called()
	return args.Error(0)
}
