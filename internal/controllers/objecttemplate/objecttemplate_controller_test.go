package objecttemplate

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
	"package-operator.run/package-operator/internal/testutil/dynamiccachemocks"
	"package-operator.run/package-operator/internal/testutil/restmappermock"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestObjectTemplateController_Reconcile(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	uncachedClient := testutil.NewClient()
	log := testr.New(t)
	dc := &dynamiccachemocks.DynamicCacheMock{}
	rm := &restmappermock.RestMapperMock{}
	controller := NewObjectTemplateController(c, uncachedClient, log, dc, testScheme, rm)
	controller.reconciler = nil // we are testing reconcilers on their own

	objectKey := client.ObjectKey{Name: "test", Namespace: "testns"}
	c.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.ObjectTemplate"), mock.Anything).
		Return(nil)
	c.
		On("Patch", mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectTemplate"), mock.Anything, mock.Anything).
		Return(nil)
	c.StatusMock.
		On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectTemplate"), mock.Anything).
		Return(nil)

	ctx := context.Background()
	res, err := controller.Reconcile(ctx, reconcile.Request{
		NamespacedName: objectKey,
	})
	require.NoError(t, err)
	assert.True(t, res.IsZero())

	dc.AssertExpectations(t)
}

func TestObjectTemplateController_Reconcile_deletion(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	uncachedClient := testutil.NewClient()
	log := testr.New(t)
	dc := &dynamiccachemocks.DynamicCacheMock{}
	rm := &restmappermock.RestMapperMock{}
	controller := NewObjectTemplateController(c, uncachedClient, log, dc, testScheme, rm)
	controller.reconciler = nil // we are testing reconcilers on their own

	objectKey := client.ObjectKey{Name: "test", Namespace: "testns"}
	c.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.ObjectTemplate"), mock.Anything).
		Run(func(args mock.Arguments) {
			now := metav1.Now()
			o := args.Get(2).(*corev1alpha1.ObjectTemplate)
			*o = corev1alpha1.ObjectTemplate{ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp: &now,
			}}
		}).
		Return(nil)
	c.
		On("Patch", mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectTemplate"), mock.Anything, mock.Anything).
		Return(nil)
	dc.
		On("Free", mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectTemplate"), mock.Anything).
		Return(nil)

	ctx := context.Background()
	res, err := controller.Reconcile(ctx, reconcile.Request{
		NamespacedName: objectKey,
	})
	require.NoError(t, err)
	assert.True(t, res.IsZero())

	dc.AssertExpectations(t)
}
