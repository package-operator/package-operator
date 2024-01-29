package hostedclusters

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	hypershiftv1beta1 "package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
	"package-operator.run/internal/testutil"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := hypershiftv1beta1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestHostedClusterController_noop(t *testing.T) {
	t.Parallel()

	mockClient := testutil.NewClient()

	image := "image321"
	controller := NewHostedClusterController(
		mockClient, ctrl.Log.WithName("hc controller test"), testScheme, image, nil, nil,
	)
	hcName := "testing123"
	now := metav1.Now()
	hc := &hypershiftv1beta1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:              hcName,
			DeletionTimestamp: &now,
		},
	}

	mockClient.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1beta1.HostedCluster"), mock.Anything).
		Run(func(args mock.Arguments) {
			obj := args.Get(2).(*hypershiftv1beta1.HostedCluster)
			*obj = *hc
		}).
		Return(nil)

	ctx := context.Background()
	res, err := controller.Reconcile(ctx, ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(hc),
	})
	require.NoError(t, err)
	assert.True(t, res.IsZero())
}

func TestHostedClusterController_DesiredPackage(t *testing.T) {
	t.Parallel()

	mockClient := testutil.NewClient()

	image := "image321"
	controller := NewHostedClusterController(mockClient, ctrl.Log.WithName("hc controller test"), testScheme, image,
		&corev1.Affinity{},
		[]corev1.Toleration{{}})
	hcName := "testing123"
	hc := &hypershiftv1beta1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: hcName},
	}

	pkg, err := controller.desiredPackage(hc, "remote-phase")
	require.NoError(t, err)
	assert.Equal(t, "remote-phase", pkg.Name)
	assert.Equal(t, image, pkg.Spec.Image)
	if assert.NotNil(t, pkg.Spec.Config) {
		assert.Equal(t, `{"affinity":{},"tolerations":[{}]}`, string(pkg.Spec.Config.Raw))
	}
}

var readyHostedCluster = &hypershiftv1beta1.HostedCluster{
	Status: hypershiftv1beta1.HostedClusterStatus{
		Conditions: []metav1.Condition{
			{Type: hypershiftv1beta1.HostedClusterAvailable, Status: metav1.ConditionTrue},
		},
	},
}

func TestHostedClusterController_Reconcile_waitsForClusterReady(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := NewHostedClusterController(
		clientMock, ctrl.Log.WithName("hc controller test"), testScheme, "desired-image:test", nil, nil,
	)

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1beta1.HostedCluster"), mock.Anything).
		Return(nil)

	clientMock.
		On("Create", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	res, err := c.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)
	assert.Empty(t, res)

	clientMock.AssertNotCalled(t, "Create", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything)
}

func TestHostedClusterController_Reconcile_createsPackage(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := NewHostedClusterController(
		clientMock, ctrl.Log.WithName("hc controller test"), testScheme, "desired-image:test", nil, nil,
	)

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1beta1.HostedCluster"), mock.Anything).
		Run(func(args mock.Arguments) {
			obj := args.Get(2).(*hypershiftv1beta1.HostedCluster)
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
	require.NoError(t, err)
	assert.Empty(t, res)

	clientMock.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything)
}

func TestHostedClusterController_Reconcile_updatesPackage(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := NewHostedClusterController(
		clientMock, ctrl.Log.WithName("hc controller test"), testScheme, "desired-image:test", nil, nil,
	)

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1beta1.HostedCluster"), mock.Anything).
		Run(func(args mock.Arguments) {
			obj := args.Get(2).(*hypershiftv1beta1.HostedCluster)
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
	require.NoError(t, err)
	assert.Empty(t, res)

	clientMock.AssertNotCalled(t, "Create", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything)
	clientMock.AssertCalled(t, "Update", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything)
}
