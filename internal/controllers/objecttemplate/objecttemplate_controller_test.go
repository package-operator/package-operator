package objecttemplate

import (
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

// func TestObjectTemplateController_Reconcile(t *testing.T) {
// 	t.Parallel()

// 	c := testutil.NewClient()
// 	uncachedClient := testutil.NewClient()
// 	log := testr.New(t)
// 	accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
// 	rm := &restmappermock.RestMapperMock{}
// 	cfg := ControllerConfig{
// 		OptionalResourceRetryInterval: time.Second * 30,
// 		ResourceRetryInterval:         time.Second * 30,
// 	}

// 	controller := NewObjectTemplateController(c, uncachedClient, log, accessManager, testScheme, rm, cfg)
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

// 	c.AssertExpectations(t)
// 	uncachedClient.AssertExpectations(t)
// 	c.StatusMock.AssertExpectations(t)
// 	accessManager.AssertExpectations(t)
// }

// func TestObjectTemplateController_Reconcile_deletion(t *testing.T) {
// 	t.Parallel()

// 	c := testutil.NewClient()
// 	uncachedClient := testutil.NewClient()
// 	log := testr.New(t)
// 	accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
// 	rm := &restmappermock.RestMapperMock{}
// 	cfg := ControllerConfig{
// 		OptionalResourceRetryInterval: time.Second * 30,
// 		ResourceRetryInterval:         time.Second * 30,
// 	}
// 	controller := NewObjectTemplateController(c, uncachedClient, log, accessManager, testScheme, rm, cfg)
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
// 	accessManager.
// 		On("FreeWithUser", mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectTemplate"), mock.Anything).
// 		Return(nil)

// 	ctx := context.Background()
// 	res, err := controller.Reconcile(ctx, reconcile.Request{
// 		NamespacedName: objectKey,
// 	})
// 	require.NoError(t, err)
// 	assert.True(t, res.IsZero())

// 	c.AssertExpectations(t)
// 	uncachedClient.AssertExpectations(t)
// 	c.StatusMock.AssertExpectations(t)
// 	accessManager.AssertExpectations(t)
// }
