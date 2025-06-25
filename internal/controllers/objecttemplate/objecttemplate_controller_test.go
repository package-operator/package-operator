package objecttemplate

// import (
// 	"context"
// 	"testing"
// 	"time"

// 	"github.com/go-logr/logr/testr"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/mock"
// 	"github.com/stretchr/testify/require"
// 	corev1 "k8s.io/api/core/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/runtime"
// 	"k8s.io/apimachinery/pkg/runtime/schema"
// 	"k8s.io/apimachinery/pkg/types"
// 	"pkg.package-operator.run/boxcutter/managedcache"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
// 	"sigs.k8s.io/controller-runtime/pkg/reconcile"

// 	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
// 	"package-operator.run/internal/testutil"
// 	"package-operator.run/internal/testutil/dynamiccachemocks"
// 	"package-operator.run/internal/testutil/restmappermock"
// )

// var testScheme = runtime.NewScheme()

// func init() {
// 	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
// 		panic(err)
// 	}
// }

// // TODO(rjurcaga): export in boxcutter instead?
// type ownerRefGetterMock struct {
// 	mock.Mock
// }

// func (m *ownerRefGetterMock) getWatchersForGVK(gvk schema.GroupVersionKind) []accessManagerKey {
// 	args := m.Called(gvk)

// 	return args.Get(0).([]accessManagerKey)
// }

// type accessManagerKey struct {
// 	// UID ensures a re-created object also gets it's own cache.
// 	UID types.UID
// 	schema.GroupVersionKind
// 	client.ObjectKey
// }

// func TestObjectTemplateController_Reconcile(t *testing.T) {
// 	t.Parallel()

// 	c := testutil.NewClient()
// 	uncachedClient := testutil.NewClient()
// 	log := testr.New(t)
// 	// dc := &dynamiccachemocks.DynamicCacheMock{}
// 	ownerRefGetter := &ownerRefGetterMock{}
// 	scheme := runtime.NewScheme()
// 	h := managedcache.NewEnqueueWatchingObjects(ownerRefGetter, &corev1.ConfigMap{}, scheme)
// 	rm := &restmappermock.RestMapperMock{}
// 	cfg := ControllerConfig{
// 		OptionalResourceRetryInterval: time.Second * 30,
// 		ResourceRetryInterval:         time.Second * 30,
// 	}
// 	controller := NewObjectTemplateController(c, uncachedClient, log, h, testScheme, rm, cfg)
// 	controller.reconciler = nil // we are testing reconcilers on their own

// 	objectKey := client.ObjectKey{Name: "test", Namespace: "testns"}
// 	c.
// 		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.ObjectTemplate"), mock.Anything).
// 		Return(nil)
// 	c.
// 		On("Patch", mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectTemplate"), mock.Anything, mock.Anything).
// 		Return(nil)
// 	c.StatusMock.
// 		On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectTemplate"), mock.Anything).
// 		Return(nil)

// 	ctx := context.Background()
// 	res, err := controller.Reconcile(ctx, reconcile.Request{
// 		NamespacedName: objectKey,
// 	})
// 	require.NoError(t, err)
// 	assert.True(t, res.IsZero())

// 	ownerRefGetter.AssertExpectations(t)
// }

// func TestObjectTemplateController_Reconcile_deletion(t *testing.T) {
// 	t.Parallel()

// 	c := testutil.NewClient()
// 	uncachedClient := testutil.NewClient()
// 	log := testr.New(t)
// 	dc := &dynamiccachemocks.DynamicCacheMock{}
// 	rm := &restmappermock.RestMapperMock{}
// 	cfg := ControllerConfig{
// 		OptionalResourceRetryInterval: time.Second * 30,
// 		ResourceRetryInterval:         time.Second * 30,
// 	}
// 	controller := NewObjectTemplateController(c, uncachedClient, log, dc, testScheme, rm, cfg)
// 	controller.reconciler = nil // we are testing reconcilers on their own

// 	objectKey := client.ObjectKey{Name: "test", Namespace: "testns"}
// 	c.
// 		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.ObjectTemplate"), mock.Anything).
// 		Run(func(args mock.Arguments) {
// 			now := metav1.Now()
// 			o := args.Get(2).(*corev1alpha1.ObjectTemplate)
// 			*o = corev1alpha1.ObjectTemplate{ObjectMeta: metav1.ObjectMeta{
// 				DeletionTimestamp: &now,
// 			}}
// 		}).
// 		Return(nil)
// 	c.
// 		On("Patch", mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectTemplate"), mock.Anything, mock.Anything).
// 		Return(nil)
// 	dc.
// 		On("Free", mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectTemplate"), mock.Anything).
// 		Return(nil)

// 	ctx := context.Background()
// 	res, err := controller.Reconcile(ctx, reconcile.Request{
// 		NamespacedName: objectKey,
// 	})
// 	require.NoError(t, err)
// 	assert.True(t, res.IsZero())

// 	dc.AssertExpectations(t)
// }
