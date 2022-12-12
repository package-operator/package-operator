package hostedclusters

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	hypershiftv1alpha1 "package-operator.run/package-operator/internal/controllers/hostedclusters/hypershift/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := hypershiftv1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestHostedClusterController_DesiredPackage(t *testing.T) {
	mockClient := testutil.NewClient()

	image := "image321"
	controller := NewHostedClusterController(mockClient, ctrl.Log.WithName("hc controller test"), testScheme, image)
	hcName := "testing123"
	hc := &hypershiftv1alpha1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: hcName},
	}

	pkg := controller.desiredPackage(hc)
	assert.Equal(t, hcName+"-remote-phase", pkg.Name)
	assert.Equal(t, image, pkg.Spec.Image)
}

var readyHostedCluster = &hypershiftv1alpha1.HostedCluster{
	Status: hypershiftv1alpha1.HostedClusterStatus{
		Conditions: []metav1.Condition{
			{Type: hypershiftv1alpha1.HostedClusterAvailable, Status: metav1.ConditionTrue},
		},
	},
}

func TestHostedClusterController_Reconcile_waitsForClusterReady(t *testing.T) {
	clientMock := testutil.NewClient()
	c := NewHostedClusterController(clientMock, ctrl.Log.WithName("hc controller test"), testScheme, "desired-image:test")

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.HostedCluster"), mock.Anything).
		Return(nil)

	clientMock.
		On("Create", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	res, err := c.Reconcile(context.Background(), ctrl.Request{})
	assert.NoError(t, err)
	assert.Empty(t, res)

	clientMock.AssertNotCalled(t, "Create", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything)
}

func TestHostedClusterController_Reconcile_createsPackage(t *testing.T) {
	clientMock := testutil.NewClient()
	c := NewHostedClusterController(clientMock, ctrl.Log.WithName("hc controller test"), testScheme, "desired-image:test")

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.HostedCluster"), mock.Anything).
		Run(func(args mock.Arguments) {
			obj := args.Get(2).(*hypershiftv1alpha1.HostedCluster)
			*obj = *readyHostedCluster
		}).
		Return(nil)

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
		Return(errors.NewNotFound(schema.GroupResource{}, ""))

	clientMock.
		On("Create", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	res, err := c.Reconcile(context.Background(), ctrl.Request{})
	assert.NoError(t, err)
	assert.Empty(t, res)

	clientMock.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything)
}

func TestHostedClusterController_Reconcile_updatesPackage(t *testing.T) {
	clientMock := testutil.NewClient()
	c := NewHostedClusterController(clientMock, ctrl.Log.WithName("hc controller test"), testScheme, "desired-image:test")

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.HostedCluster"), mock.Anything).
		Run(func(args mock.Arguments) {
			obj := args.Get(2).(*hypershiftv1alpha1.HostedCluster)
			*obj = *readyHostedCluster
		}).
		Return(nil)

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
		Run(func(args mock.Arguments) {
			obj := args.Get(2).(*corev1alpha1.Package)
			*obj = corev1alpha1.Package{
				Spec: corev1alpha1.PackageSpec{
					Image: "outdated-image:test",
				},
			}
		}).
		Return(nil)

	clientMock.
		On("Create", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	clientMock.
		On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	res, err := c.Reconcile(context.Background(), ctrl.Request{})
	assert.NoError(t, err)
	assert.Empty(t, res)

	clientMock.AssertNotCalled(t, "Create", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything)
	clientMock.AssertCalled(t, "Update", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything)
}
