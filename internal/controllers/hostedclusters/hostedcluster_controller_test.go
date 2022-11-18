package hostedclusters

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers/hostedclusters/hypershift/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
)

func TestHostedClusterController_DesiredPackage(t *testing.T) {
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	mockClient := testutil.NewClient()

	image := "image321"
	controller := NewHostedClusterController(mockClient, ctrl.Log.WithName("hc controller test"), scheme, image)
	hcName := "testing123"
	hc := &v1alpha1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: hcName},
	}

	pkg := controller.desiredPackage(hc)
	assert.Equal(t, hcName+"_remote_phase_manager", pkg.Name)
	assert.Equal(t, image, pkg.Spec.Image)
}

func TestHostedClusterController_Reconcile(t *testing.T) {
	tests := []struct {
		name             string
		hcCondition      metav1.Condition
		hcGetReturn      error
		pkgGetReturn     error
		pkgImage         string
		existingPkgImage string
	}{
		{
			name:        "hostedcluster deleted",
			hcGetReturn: errors.NewNotFound(schema.GroupResource{}, ""),
		},
		{
			name:        "hostedcluster unavailable",
			hcCondition: metav1.Condition{Type: v1alpha1.HostedClusterAvailable, Status: "False"},
		},
		{
			name:         "no existing package",
			hcCondition:  metav1.Condition{Type: v1alpha1.HostedClusterAvailable, Status: metav1.ConditionTrue},
			pkgGetReturn: errors.NewNotFound(schema.GroupResource{}, ""),
			pkgImage:     "image",
		},
		{
			name:             "existing package with same image",
			hcCondition:      metav1.Condition{Type: v1alpha1.HostedClusterAvailable, Status: metav1.ConditionTrue},
			pkgImage:         "same-image",
			existingPkgImage: "same-image",
		},
		{
			name:             "existing package with different image",
			hcCondition:      metav1.Condition{Type: v1alpha1.HostedClusterAvailable, Status: metav1.ConditionTrue},
			pkgImage:         "image",
			existingPkgImage: "different-image",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
			err := v1alpha1.AddToScheme(scheme)
			require.NoError(t, err)
			clientMock := testutil.NewClient()
			clientMock.
				On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					obj := args.Get(2).(*v1alpha1.HostedCluster)
					*obj = v1alpha1.HostedCluster{
						Status: v1alpha1.HostedClusterStatus{
							Conditions: []metav1.Condition{test.hcCondition},
						},
					}
				}).
				Return(test.hcGetReturn).Once()

			clientMock.
				On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					obj := args.Get(2).(*corev1alpha1.Package)
					*obj = corev1alpha1.Package{
						Spec: corev1alpha1.PackageSpec{
							Image: test.existingPkgImage,
						},
					}
				}).
				Return(test.pkgGetReturn).Maybe().Once()

			clientMock.
				On("Create", mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe().Once()
			clientMock.
				On("Delete", mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe().Once()

			controller := NewHostedClusterController(clientMock, ctrl.Log.WithName("hc controller test"), scheme, test.pkgImage)

			res, err := controller.Reconcile(context.Background(), ctrl.Request{})
			assert.NoError(t, err)
			assert.Empty(t, res)

			if test.hcGetReturn != nil || test.hcCondition.Status == "False" {
				clientMock.AssertNumberOfCalls(t, "Get", 1)
				clientMock.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
				clientMock.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
			}

			if test.pkgGetReturn != nil {
				clientMock.AssertNumberOfCalls(t, "Get", 2)
				clientMock.AssertCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
				clientMock.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
			}

			if test.hcCondition.Status == metav1.ConditionTrue && test.pkgGetReturn == nil {
				clientMock.AssertNumberOfCalls(t, "Get", 2)
				if test.pkgImage == test.existingPkgImage {
					clientMock.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
					clientMock.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
				} else {
					clientMock.AssertCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
					clientMock.AssertCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
				}

			}
		})
	}

}

func TestIsHostedClusterReady(t *testing.T) {
	tests := []struct {
		name  string
		cond  metav1.Condition
		ready bool
	}{
		{
			name:  "Available condition true",
			cond:  metav1.Condition{Type: v1alpha1.HostedClusterAvailable, Status: metav1.ConditionTrue},
			ready: true,
		},
		{
			name:  "Available condition true",
			cond:  metav1.Condition{Type: v1alpha1.HostedClusterAvailable, Status: metav1.ConditionFalse},
			ready: false,
		},
		{
			name:  "Empty condition",
			cond:  metav1.Condition{},
			ready: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			hc := &v1alpha1.HostedCluster{
				Status: v1alpha1.HostedClusterStatus{Conditions: []metav1.Condition{test.cond}},
			}
			ready := isHostedClusterReady(hc)

			assert.Equal(t, test.ready, ready)
		})
	}
}
