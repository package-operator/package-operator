package dynamicwatcher

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	"package-operator.run/package-operator/internal/testutil"
)

func TestDynamicWatcher(t *testing.T) {
	log := testutil.NewLogger(t)
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	restMapper := &RestMapperMock{}
	dynamicClient := testutil.NewDynamicClient()
	secretDC := &testutil.DynamicClientNamespaceableResourceInterface{}

	secretsGVR := schema.GroupVersionResource{
		Version:  "v1",
		Resource: "secrets",
	}

	dynamicClient.
		On("Resource", secretsGVR).
		Return(secretDC)

	informerMock := &InformerMock{}
	informerMock.On("Run", mock.Anything)
	informerMock.On("AddEventHandler", mock.Anything)

	restMapper.
		On("RESTMapping", schema.GroupKind{Kind: "Secret"}, []string{"v1"}).
		Return(&meta.RESTMapping{Resource: secretsGVR}, nil)

	dw := New(log, scheme, restMapper, dynamicClient,
		EventHandlerNewInformer(func(
			lw toolscache.ListerWatcher, exampleObject runtime.Object,
			defaultEventHandlerResyncPeriod time.Duration, indexers toolscache.Indexers,
		) informer {
			return informerMock
		}))

	assert.Equal(t, dw.String(), "DynamicWatcher")

	// Register handler
	require.NoError(t, dw.Start(context.Background(), nil, nil, nil))

	// Start a new Watch
	owner := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "testns",
		},
	}
	require.NoError(t, dw.Watch(owner, &corev1.Secret{}))
	restMapper.AssertExpectations(t)
	informerMock.AssertCalled(t, "AddEventHandler", mock.Anything)
	dynamicClient.AssertExpectations(t)

	// List Owners watching secrets
	ngkv := NamespacedGKV{
		GroupVersionKind: schema.GroupVersionKind{
			Kind:    "Secret",
			Version: "v1",
		},
		Namespace: owner.Namespace,
	}
	owners := dw.OwnersForNamespacedGKV(ngkv)
	if assert.Len(t, owners, 1) {
		assert.Equal(t, OwnerReference{
			GroupKind: schema.GroupKind{
				Kind: "ConfigMap",
			},
			Name:      owner.Name,
			Namespace: owner.Namespace,
		}, owners[0])
	}

	// Free watches
	require.NoError(t, dw.Free(owner))

	// List Owners watching secrets again
	owners2 := dw.OwnersForNamespacedGKV(ngkv)
	assert.Len(t, owners2, 0)
}

type RestMapperMock struct {
	mock.Mock
}

func (rm *RestMapperMock) RESTMapping(
	gk schema.GroupKind, versions ...string,
) (*meta.RESTMapping, error) {
	args := rm.Called(gk, versions)
	return args.Get(0).(*meta.RESTMapping), args.Error(1)
}

type InformerMock struct {
	mock.Mock
}

var _ informer = (*InformerMock)(nil)

func (i *InformerMock) AddEventHandler(handler toolscache.ResourceEventHandler) {
	i.Called(handler)
}

func (i *InformerMock) AddEventHandlerWithResyncPeriod(handler toolscache.ResourceEventHandler, resyncPeriod time.Duration) {
	i.Called(handler, resyncPeriod)
}

func (i *InformerMock) AddIndexers(indexers toolscache.Indexers) error {
	args := i.Called(indexers)
	return args.Error(0)
}
func (i *InformerMock) HasSynced() bool {
	args := i.Called()
	return args.Bool(0)
}

func (i *InformerMock) Run(stopCh <-chan struct{}) {
	i.Called(stopCh)
}
