package managedcachemocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"pkg.package-operator.run/boxcutter/managedcache"
)

var _ managedcache.ObjectBoundAccessManager[client.Object] = (*ObjectBoundAccessManagerMock[client.Object])(nil)

type ObjectBoundAccessManagerMock[T managedcache.RefType] struct {
	mock.Mock
}

func (m *ObjectBoundAccessManagerMock[T]) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *ObjectBoundAccessManagerMock[T]) Get(ctx context.Context, owner T) (managedcache.Accessor, error) {
	args := m.Called(ctx, owner)
	return args.Get(0).(managedcache.Accessor), args.Error(1)
}

func (m *ObjectBoundAccessManagerMock[T]) GetWithUser(
	ctx context.Context, owner T,
	user client.Object, usedFor []client.Object,
) (managedcache.Accessor, error) {
	args := m.Called(ctx, owner, user, usedFor)
	return args.Get(0).(managedcache.Accessor), args.Error(1)
}

func (m *ObjectBoundAccessManagerMock[T]) Free(ctx context.Context, owner T) error {
	args := m.Called(ctx, owner)
	return args.Error(0)
}

func (m *ObjectBoundAccessManagerMock[T]) FreeWithUser(ctx context.Context, owner T, user client.Object) error {
	args := m.Called(ctx, owner, user)
	return args.Error(0)
}

func (m *ObjectBoundAccessManagerMock[T]) Source(
	handler handler.EventHandler,
	predicates ...predicate.Predicate,
) source.Source {
	args := m.Called(handler, predicates)
	return args.Get(0).(source.Source)
}

func (m *ObjectBoundAccessManagerMock[T]) GetWatchersForGVK(
	gvk schema.GroupVersionKind,
) []managedcache.AccessManagerKey {
	args := m.Called(gvk)
	return args.Get(0).([]managedcache.AccessManagerKey)
}
