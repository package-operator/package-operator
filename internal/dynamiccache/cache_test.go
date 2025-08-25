package dynamiccache

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"package-operator.run/internal/testutil/metricsmocks"
)

func TestCache_Start(t *testing.T) {
	t.Parallel()
	c, cacheSource, _ := setupTestCache(t)

	cacheSource.On("blockNewRegistrations")

	ctx := context.Background()
	err := c.Start(ctx)
	require.NoError(t, err)

	cacheSource.AssertCalled(t, "blockNewRegistrations")
}

func TestCache_OwnersForGVK(t *testing.T) {
	t.Parallel()
	c, _, _ := setupTestCache(t)

	owner1 := OwnerReference{
		GroupKind: schema.GroupKind{
			Kind:  "TestParent",
			Group: "testing.package-operator.run",
		},
		Name:      "test23",
		Namespace: "test",
	}
	owner2 := OwnerReference{
		GroupKind: schema.GroupKind{
			Kind:  "TestParent",
			Group: "testing.package-operator.run",
		},
		Name: "test23",
	}
	gvk := schema.GroupVersionKind{
		Kind:    "Test",
		Version: "v1beta3",
		Group:   "testing.package-operator.run",
	}
	c.informerReferences[gvk] = map[OwnerReference]struct{}{
		owner1: {},
		owner2: {},
	}

	if owners := c.OwnersForGKV(gvk); assert.Len(t, owners, 2) {
		// test with contains, because order is not guaranteed.
		assert.Contains(t, owners, owner1)
		assert.Contains(t, owners, owner2)
	}
}

func TestCache_Watch(t *testing.T) {
	t.Parallel()
	t.Run("default", func(t *testing.T) {
		t.Parallel()
		c, cacheSource, informerMap := setupTestCache(t)

		informerMap.
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil, nil)
		cacheSource.On("handleNewInformer", mock.Anything).Return(nil)

		// Use context with logger instead of plain background context
		ctx := testContextWithLogger(t)
		owner := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test42",
				Namespace: "test",
			},
		}
		obj := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test42",
				Namespace: "test",
			},
		}
		err := c.Watch(ctx, owner, obj)
		require.NoError(t, err)

		informerMap.AssertCalled(t, "Get", mock.Anything, schema.GroupVersionKind{
			Kind:    "Secret",
			Version: "v1",
		}, mock.IsType(&unstructured.Unstructured{}))
		cacheSource.AssertCalled(t, "handleNewInformer", mock.Anything)
	})

	t.Run("informer exists", func(t *testing.T) {
		t.Parallel()
		c, cacheSource, informerMap := setupTestCache(t)
		c.informerReferences[schema.GroupVersionKind{
			Kind:    "Secret",
			Version: "v1",
		}] = map[OwnerReference]struct{}{}

		informerMap.
			On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil, nil)
		cacheSource.On("handleNewInformer", mock.Anything).Return(nil)

		// Use context with logger instead of plain background context
		ctx := testContextWithLogger(t)
		owner := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test42",
				Namespace: "test",
			},
		}
		obj := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test42",
				Namespace: "test",
			},
		}
		err := c.Watch(ctx, owner, obj)
		require.NoError(t, err)

		informerMap.AssertNotCalled(t, "Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		cacheSource.AssertNotCalled(t, "handleNewInformer", mock.Anything)
	})
}

func TestCache_Free(t *testing.T) {
	t.Parallel()
	c, _, informerMap := setupTestCache(t)
	owner := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test42",
			Namespace: "test",
		},
	}
	ref, err := c.ownerRef(owner)
	require.NoError(t, err)
	c.informerReferences[schema.GroupVersionKind{
		Kind:    "Secret",
		Version: "v1",
	}] = map[OwnerReference]struct{}{
		ref: {},
	}
	informerMap.
		On("Delete", mock.Anything, mock.Anything).
		Return(nil)

	// Use context with logger instead of plain background context
	ctx := testContextWithLogger(t)
	err = c.Free(ctx, owner)
	require.NoError(t, err)

	informerMap.AssertCalled(t, "Delete", mock.Anything, schema.GroupVersionKind{
		Kind:    "Secret",
		Version: "v1",
	})
}

//nolint:paralleltest
func TestCache_Reader(t *testing.T) {
	c, _, informerMap := setupTestCache(t)
	owner := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test42", Namespace: "test"},
	}
	ref, err := c.ownerRef(owner)
	require.NoError(t, err)
	c.informerReferences[schema.GroupVersionKind{Kind: "Secret", Version: "v1"}] = map[OwnerReference]struct{}{ref: {}}

	reader := &readerMock{}
	reader.
		On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	reader.
		On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	informerMap.
		On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, reader, nil)

	t.Run("Get", func(t *testing.T) {
		obj := &corev1.Secret{}

		ctx := context.Background()
		key := client.ObjectKey{Name: "test42", Namespace: "test"}
		err = c.Get(ctx, key, obj)
		require.NoError(t, err)

		reader.AssertCalled(t, "Get", mock.Anything, key, mock.IsType(&unstructured.Unstructured{}), mock.Anything)
	})

	t.Run("List", func(t *testing.T) {
		obj := &corev1.SecretList{}

		ctx := context.Background()
		err = c.List(ctx, obj)
		require.NoError(t, err)

		reader.AssertCalled(t, "List", mock.Anything, mock.IsType(&unstructured.UnstructuredList{}), mock.Anything)
	})

	// "reset" informerReferences to test error case,
	// when no informer has been registered beforehand.
	c.informerReferences = map[schema.GroupVersionKind]map[OwnerReference]struct{}{}

	t.Run("Get no informer", func(t *testing.T) {
		obj := &corev1.Secret{}

		ctx := context.Background()
		key := client.ObjectKey{Name: "test42", Namespace: "test"}
		err = c.Get(ctx, key, obj)
		require.ErrorIs(t, err, &CacheNotStartedError{})
	})

	t.Run("List no informer", func(t *testing.T) {
		obj := &corev1.SecretList{}

		ctx := context.Background()
		err = c.List(ctx, obj)
		require.ErrorIs(t, err, &CacheNotStartedError{})
	})
}

func TestCache_sampleMetrics(t *testing.T) {
	t.Parallel()
	c, _, informerMap := setupTestCache(t)
	recorderMock := &metricsmocks.RecorderMock{}
	c.recorder = recorderMock
	secretGVK := schema.GroupVersionKind{
		Version: "v1",
		Kind:    "Secret",
	}
	configMapGVK := schema.GroupVersionKind{
		Version: "v1",
		Kind:    "ConfigMap",
	}
	c.informerReferences[secretGVK] = map[OwnerReference]struct{}{}
	c.informerReferences[configMapGVK] = map[OwnerReference]struct{}{}

	recorderMock.On("RecordDynamicCacheInformers", mock.Anything)
	recorderMock.On("RecordDynamicCacheObjects", mock.Anything, mock.Anything)

	reader := &readerMock{}
	reader.
		On("List", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			list := args.Get(1).(*unstructured.UnstructuredList)
			if list.GroupVersionKind().Kind == "SecretList" {
				list.Items = make([]unstructured.Unstructured, 2)
			} else {
				// ConfigMap
				list.Items = make([]unstructured.Unstructured, 1)
			}
		}).
		Return(nil)
	informerMap.
		On("Get", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, reader, nil)

	// Use context with logger instead of plain background context
	ctx := testContextWithLogger(t)

	c.sampleMetrics(ctx)
	recorderMock.AssertCalled(t, "RecordDynamicCacheInformers", 2)
	recorderMock.AssertCalled(t, "RecordDynamicCacheObjects", secretGVK, 2)
	recorderMock.AssertCalled(t, "RecordDynamicCacheObjects", configMapGVK, 1)
}

func setupTestCache(t *testing.T) (*Cache, *cacheSourceMock, *informerMapMock) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	cacheSource := &cacheSourceMock{}
	informerMap := &informerMapMock{}

	c := &Cache{
		scheme:             scheme,
		cacheSource:        cacheSource,
		informerMap:        informerMap,
		informerReferences: map[schema.GroupVersionKind]map[OwnerReference]struct{}{},
	}
	return c, cacheSource, informerMap
}

func testContextWithLogger(t *testing.T) context.Context {
	t.Helper()
	return logr.NewContext(context.Background(), testr.New(t))
}

var (
	_ informerMap  = (*informerMapMock)(nil)
	_ cacheSourcer = (*cacheSourceMock)(nil)
)

type informerMapMock struct {
	mock.Mock
}

func (m *informerMapMock) Get(
	ctx context.Context,
	gvk schema.GroupVersionKind,
	obj runtime.Object,
) (informer cache.SharedIndexInformer, reader client.Reader, err error) {
	args := m.Called(ctx, gvk, obj)
	if i := args.Get(0); i != nil {
		informer = i.(cache.SharedIndexInformer)
	}
	if r := args.Get(1); r != nil {
		reader = r.(client.Reader)
	}
	return informer, reader,
		args.Error(2)
}

func (m *informerMapMock) Delete(
	ctx context.Context,
	gvk schema.GroupVersionKind,
) error {
	args := m.Called(ctx, gvk)
	return args.Error(0)
}

type cacheSourceMock struct {
	mock.Mock
}

func (m *cacheSourceMock) Source(handler handler.EventHandler, predicates ...predicate.Predicate) source.Source {
	args := m.Called(handler, predicates)
	return args.Get(0).(source.Source)
}

func (m *cacheSourceMock) blockNewRegistrations() {
	m.Called()
}

func (m *cacheSourceMock) handleNewInformer(informer cache.SharedIndexInformer) error {
	args := m.Called(informer)
	return args.Error(0)
}

type readerMock struct {
	mock.Mock
}

var _ client.Reader = (*readerMock)(nil)

func (m *readerMock) Get(
	ctx context.Context,
	key client.ObjectKey, out client.Object,
	opts ...client.GetOption,
) error {
	args := m.Called(ctx, key, out, opts)
	return args.Error(0)
}

func (m *readerMock) List(
	ctx context.Context,
	out client.ObjectList, opts ...client.ListOption,
) error {
	args := m.Called(ctx, out, opts)
	return args.Error(0)
}
