package hostedclusterpackages

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

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

func TestHostedClusterPackageController_Reconcile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupClient    func(*testutil.CtrlClient)
		expectedResult ctrl.Result
		expectedError  string
	}{
		{
			name: "HostedClusterPackage not found",
			setupClient: func(c *testutil.CtrlClient) {
				c.On("Get",
					mock.Anything,
					types.NamespacedName{Name: "test-hcpkg"},
					mock.AnythingOfType("*v1alpha1.HostedClusterPackage"),
					mock.Anything,
				).Return(errors.NewNotFound(schema.GroupResource{}, "test-hcpkg"))
			},
			expectedResult: ctrl.Result{},
		},
		{
			name: "HostedClusterPackage being deleted",
			setupClient: func(c *testutil.CtrlClient) {
				hcpkg := &corev1alpha1.HostedClusterPackage{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-hcpkg",
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
					},
				}
				c.On("Get",
					mock.Anything,
					types.NamespacedName{Name: "test-hcpkg"},
					mock.AnythingOfType("*v1alpha1.HostedClusterPackage"),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(2).(*corev1alpha1.HostedClusterPackage)
					*arg = *hcpkg
				}).Return(nil)
			},
			expectedResult: ctrl.Result{},
		},
		{
			name: "Error listing hosted clusters",
			setupClient: func(c *testutil.CtrlClient) {
				hcpkg := &corev1alpha1.HostedClusterPackage{
					ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				}
				c.On("Get",
					mock.Anything,
					types.NamespacedName{Name: "test-hcpkg"},
					mock.AnythingOfType("*v1alpha1.HostedClusterPackage"),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(2).(*corev1alpha1.HostedClusterPackage)
					*arg = *hcpkg
				}).Return(nil)

				c.On("List",
					mock.Anything,
					mock.AnythingOfType("*v1beta1.HostedClusterList"),
					mock.Anything,
				).Return(assert.AnError)
			},
			expectedResult: ctrl.Result{},
			expectedError:  "listing clusters: assert.AnError general error for testing",
		},
		{
			name: "Successfully reconcile with no hosted clusters",
			setupClient: func(c *testutil.CtrlClient) {
				hcpkg := &corev1alpha1.HostedClusterPackage{
					ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				}
				c.On("Get",
					mock.Anything,
					types.NamespacedName{Name: "test-hcpkg"},
					mock.AnythingOfType("*v1alpha1.HostedClusterPackage"),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(2).(*corev1alpha1.HostedClusterPackage)
					*arg = *hcpkg
				}).Return(nil)

				c.On("List",
					mock.Anything,
					mock.AnythingOfType("*v1beta1.HostedClusterList"),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(1).(*hypershiftv1beta1.HostedClusterList)
					*arg = hypershiftv1beta1.HostedClusterList{Items: []hypershiftv1beta1.HostedCluster{}}
				}).Return(nil)

				c.On("List",
					mock.Anything,
					mock.AnythingOfType("*v1alpha1.PackageList"),
					mock.Anything).
					Return(nil)

				c.StatusMock.
					On("Update",
						mock.Anything,
						mock.AnythingOfType("*v1alpha1.HostedClusterPackage"),
						mock.Anything).
					Return(nil)
			},
			expectedResult: ctrl.Result{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			clientMock := testutil.NewClient()
			tt.setupClient(clientMock)

			controller := NewHostedClusterPackageController(clientMock, logr.Discard(), testScheme)
			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "test-hcpkg"}}

			result, err := controller.Reconcile(context.Background(), req)

			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}

			clientMock.AssertExpectations(t)
		})
	}
}

func TestHostedClusterPackageController_reconcileHostedCluster(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		hostedCluster hypershiftv1beta1.HostedCluster
		setupClient   func(*testutil.CtrlClient)
		expectedError string
	}{
		{
			name: "HostedCluster not available",
			hostedCluster: hypershiftv1beta1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Status: hypershiftv1beta1.HostedClusterStatus{
					Conditions: []metav1.Condition{},
				},
			},
			setupClient: func(_ *testutil.CtrlClient) {},
		},
		{
			name: "Error creating Package",
			hostedCluster: hypershiftv1beta1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Status: hypershiftv1beta1.HostedClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:   hypershiftv1beta1.HostedClusterAvailable,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			setupClient: func(c *testutil.CtrlClient) {
				c.On("Get",
					mock.Anything,
					mock.AnythingOfType("types.NamespacedName"),
					mock.AnythingOfType("*v1alpha1.Package"),
					mock.Anything,
				).Return(errors.NewNotFound(schema.GroupResource{}, "test-package"))

				c.On("Create",
					mock.Anything,
					mock.AnythingOfType("*v1alpha1.Package"),
					mock.Anything,
				).Return(assert.AnError)
			},
			expectedError: "creating Package: assert.AnError general error for testing",
		},
		{
			name: "Successfully create Package",
			hostedCluster: hypershiftv1beta1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Status: hypershiftv1beta1.HostedClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:   hypershiftv1beta1.HostedClusterAvailable,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			setupClient: func(c *testutil.CtrlClient) {
				c.On("Get",
					mock.Anything,
					mock.AnythingOfType("types.NamespacedName"),
					mock.AnythingOfType("*v1alpha1.Package"),
					mock.Anything,
				).Return(errors.NewNotFound(schema.GroupResource{}, "test-package"))

				c.On("Create",
					mock.Anything,
					mock.AnythingOfType("*v1alpha1.Package"),
					mock.Anything,
				).Return(nil)
			},
		},
		{
			name: "Successfully update existing Package",
			hostedCluster: hypershiftv1beta1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Status: hypershiftv1beta1.HostedClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:   hypershiftv1beta1.HostedClusterAvailable,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			setupClient: func(c *testutil.CtrlClient) {
				existingPkg := &corev1alpha1.Package{
					Spec: corev1alpha1.PackageSpec{Image: "old-image"},
				}
				c.On("Get",
					mock.Anything,
					mock.AnythingOfType("types.NamespacedName"),
					mock.AnythingOfType("*v1alpha1.Package"),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(2).(*corev1alpha1.Package)
					*arg = *existingPkg
				}).Return(nil)

				c.On("Update",
					mock.Anything,
					mock.AnythingOfType("*v1alpha1.Package"),
					mock.Anything,
				).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			clientMock := &testutil.CtrlClient{}
			tt.setupClient(clientMock)

			controller := NewHostedClusterPackageController(clientMock, logr.Discard(), testScheme)
			hcpkg := &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg"},
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Template: corev1alpha1.PackageTemplateSpec{
						Spec: corev1alpha1.PackageSpec{Image: "test-image"},
					},
				},
			}

			err := controller.reconcileHostedCluster(context.Background(), hcpkg, tt.hostedCluster)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}

			clientMock.AssertExpectations(t)
		})
	}
}

func TestHostedClusterPackageController_constructClusterPackage(t *testing.T) {
	t.Parallel()

	hostedClusterPackage := &corev1alpha1.HostedClusterPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-hcpkg",
			UID:  "test-uid",
		},
		Spec: corev1alpha1.HostedClusterPackageSpec{
			Template: corev1alpha1.PackageTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"test": "label"},
				},
				Spec: corev1alpha1.PackageSpec{
					Image: "test-image",
				},
			},
		},
	}
	hostedCluster := hypershiftv1beta1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hc",
			Namespace: "default",
		},
	}

	controller := NewHostedClusterPackageController(nil, logr.Discard(), testScheme)

	pkg, err := controller.constructClusterPackage(hostedClusterPackage, hostedCluster)

	require.NoError(t, err)
	require.NotNil(t, pkg)

	assert.Equal(t, hostedClusterPackage.Name, pkg.Name)
	assert.Equal(t, hypershiftv1beta1.HostedClusterNamespace(hostedCluster), pkg.Namespace)
	assert.Equal(t, hostedClusterPackage.Spec.Template.Spec, pkg.Spec)
	assert.Equal(t, hostedClusterPackage.Spec.Template.Labels, pkg.Labels)

	assert.Len(t, pkg.OwnerReferences, 1)
	assert.Equal(t, hostedClusterPackage.Name, pkg.OwnerReferences[0].Name)
	assert.Equal(t, hostedClusterPackage.UID, pkg.OwnerReferences[0].UID)
}

func TestHostedClusterPackageController_statusCounts(t *testing.T) {
	t.Parallel()

	type want struct {
		available   int32
		ready       int32
		packages    int32
		unavailable int32
		updated     int32
	}

	tests := []struct {
		name           string
		pkgs           []corev1alpha1.Package
		hostedClusters int
		want           want
	}{
		{
			name:           "zero packages zero clusters",
			pkgs:           nil,
			hostedClusters: 0,
			want: want{
				available:   0,
				ready:       0,
				packages:    0,
				unavailable: 0,
				updated:     0,
			},
		},
		{
			name:           "zero packages five clusters",
			pkgs:           nil,
			hostedClusters: 5,
			want: want{
				available:   0,
				ready:       0,
				packages:    0,
				unavailable: 5,
				updated:     0,
			},
		},
		{
			name: "one package no conditions, five clusters",
			pkgs: []corev1alpha1.Package{
				{Spec: corev1alpha1.PackageSpec{Image: "test-image"}},
			},
			hostedClusters: 5,
			want: want{
				available:   0,
				ready:       0,
				packages:    1,
				unavailable: 5,
				updated:     0,
			},
		},
		{
			name: "one available package, five clusters",
			pkgs: []corev1alpha1.Package{
				{
					Spec: corev1alpha1.PackageSpec{Image: "test-image"},
					Status: corev1alpha1.PackageStatus{
						Conditions: []metav1.Condition{
							{Type: corev1alpha1.PackageAvailable, Status: metav1.ConditionTrue},
						},
					},
				},
			},
			hostedClusters: 5,
			want: want{
				available:   1,
				ready:       0,
				packages:    1,
				unavailable: 4,
				updated:     0,
			},
		},
		{
			name: "one ready package, five clusters",
			pkgs: []corev1alpha1.Package{
				{
					Spec: corev1alpha1.PackageSpec{Image: "test-image"},
					Status: corev1alpha1.PackageStatus{
						Conditions: []metav1.Condition{
							{Type: corev1alpha1.PackageAvailable, Status: metav1.ConditionTrue},
							{Type: corev1alpha1.PackageUnpacked, Status: metav1.ConditionTrue},
							{Type: corev1alpha1.PackageProgressing, Status: metav1.ConditionFalse},
						},
					},
				},
			},
			hostedClusters: 5,
			want: want{
				available:   1,
				ready:       1,
				packages:    1,
				unavailable: 4,
				updated:     0,
			},
		},
		{
			name: "ne upgraded package, five clusters",
			pkgs: []corev1alpha1.Package{
				{
					Spec: corev1alpha1.PackageSpec{Image: "hostedclusterpackage-image"},
					Status: corev1alpha1.PackageStatus{
						Conditions: []metav1.Condition{
							{Type: corev1alpha1.PackageAvailable, Status: metav1.ConditionTrue},
						},
					},
				},
			},
			hostedClusters: 5,
			want: want{
				available:   1,
				ready:       0,
				packages:    1,
				unavailable: 4,
				updated:     1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			counts := &corev1alpha1.HostedClusterPackageCountsStatus{}
			hostedPackage := &corev1alpha1.HostedClusterPackage{
				Spec: corev1alpha1.HostedClusterPackageSpec{
					Template: corev1alpha1.PackageTemplateSpec{
						Spec: corev1alpha1.PackageSpec{
							Image: "hostedclusterpackage-image",
						},
					},
				},
			}
			updateStatusCounts(counts, hostedPackage, tt.pkgs, tt.hostedClusters)

			assert.Equal(t, tt.want.available, counts.AvailablePackages)
			assert.Equal(t, tt.want.ready, counts.ReadyPackages)
			assert.Equal(t, tt.want.packages, counts.Packages)
			assert.Equal(t, tt.want.unavailable, counts.UnavailablePackages)
			assert.Equal(t, tt.want.updated, counts.UpdatedPackages)
		})
	}
}
