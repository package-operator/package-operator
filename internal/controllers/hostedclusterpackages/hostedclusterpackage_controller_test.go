package hostedclusterpackages

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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

//nolint:maintidx
func TestHostedClusterPackageController_Reconcile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupClient    func(client *testutil.CtrlClient)
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

				c.StatusMock.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.HostedClusterPackage{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					hcpkg := args.Get(1).(*corev1alpha1.HostedClusterPackage)

					// Validate counts
					assert.Equal(t, int32(0), hcpkg.Status.ObservedGeneration)
					assert.Equal(t, int32(0), hcpkg.Status.TotalPackages)
					assert.Equal(t, int32(0), hcpkg.Status.AvailablePackages)
					assert.Equal(t, int32(0), hcpkg.Status.ProgressedPackages)
					assert.Equal(t, int32(0), hcpkg.Status.UpdatedPackages)

					// Validate conditions
					require.Len(t, hcpkg.Status.Conditions, 2)

					availableCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageAvailable)
					require.NotNil(t, availableCond)
					assert.Equal(t, metav1.ConditionTrue, availableCond.Status)
					assert.Equal(t, int64(0), availableCond.ObservedGeneration)
					assert.Equal(t, "0/0 packages available.", availableCond.Message)

					progressingCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageProgressing)
					require.NotNil(t, progressingCond)
					assert.Equal(t, metav1.ConditionTrue, progressingCond.Status)
					assert.Equal(t, int64(0), progressingCond.ObservedGeneration)
					assert.Equal(t, "0/0 packages progressed.", progressingCond.Message)
				}).Return(nil)
			},
			expectedResult: ctrl.Result{},
		},
		{
			name: "All 5 packages available and progressed",
			setupClient: func(c *testutil.CtrlClient) {
				hcpkg := &corev1alpha1.HostedClusterPackage{
					ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg", Generation: 1},
					Spec: corev1alpha1.HostedClusterPackageSpec{
						Strategy: corev1alpha1.HostedClusterPackageStrategy{
							RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
								MaxUnavailable: 1,
							},
						},
						Template: corev1alpha1.PackageTemplateSpec{
							Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
						},
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

				hostedClusters := makeHostedClusters()
				c.On("List",
					mock.Anything,
					mock.AnythingOfType("*v1beta1.HostedClusterList"),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(1).(*hypershiftv1beta1.HostedClusterList)
					*arg = hypershiftv1beta1.HostedClusterList{Items: hostedClusters}
				}).Return(nil)

				for i := range 5 {
					pkg := makePackage(i, true, true)
					c.On("Get",
						mock.Anything,
						types.NamespacedName{Name: "test-hcpkg", Namespace: fmt.Sprintf("default-hc-%d", i)},
						mock.AnythingOfType("*v1alpha1.Package"),
						mock.Anything,
					).Run(func(args mock.Arguments) {
						arg := args.Get(2).(*corev1alpha1.Package)
						*arg = pkg
					}).Return(nil)
				}

				c.StatusMock.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.HostedClusterPackage{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					hcpkg := args.Get(1).(*corev1alpha1.HostedClusterPackage)

					// Validate counts
					assert.Equal(t, int32(1), hcpkg.Status.ObservedGeneration)
					assert.Equal(t, int32(5), hcpkg.Status.TotalPackages)
					assert.Equal(t, int32(5), hcpkg.Status.AvailablePackages)
					assert.Equal(t, int32(5), hcpkg.Status.ProgressedPackages)
					assert.Equal(t, int32(5), hcpkg.Status.UpdatedPackages)

					// Validate conditions
					require.Len(t, hcpkg.Status.Conditions, 2)

					availableCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageAvailable)
					require.NotNil(t, availableCond)
					assert.Equal(t, metav1.ConditionTrue, availableCond.Status)
					assert.Equal(t, int64(1), availableCond.ObservedGeneration)
					assert.Equal(t, "5/5 packages available.", availableCond.Message)

					progressingCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageProgressing)
					require.NotNil(t, progressingCond)
					assert.Equal(t, metav1.ConditionTrue, progressingCond.Status)
					assert.Equal(t, int64(1), progressingCond.ObservedGeneration)
					assert.Equal(t, "5/5 packages progressed.", progressingCond.Message)
				}).Return(nil)
			},
			expectedResult: ctrl.Result{},
		},
		{
			name: "All 5 packages available, none progressed",
			setupClient: func(c *testutil.CtrlClient) {
				hcpkg := &corev1alpha1.HostedClusterPackage{
					ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg", Generation: 1},
					Spec: corev1alpha1.HostedClusterPackageSpec{
						Strategy: corev1alpha1.HostedClusterPackageStrategy{
							RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
								MaxUnavailable: 1,
							},
						},
						Template: corev1alpha1.PackageTemplateSpec{
							Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
						},
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

				hostedClusters := makeHostedClusters()
				c.On("List",
					mock.Anything,
					mock.AnythingOfType("*v1beta1.HostedClusterList"),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(1).(*hypershiftv1beta1.HostedClusterList)
					*arg = hypershiftv1beta1.HostedClusterList{Items: hostedClusters}
				}).Return(nil)

				for i := range 5 {
					pkg := makePackage(i, true, false)
					c.On("Get",
						mock.Anything,
						types.NamespacedName{Name: "test-hcpkg", Namespace: fmt.Sprintf("default-hc-%d", i)},
						mock.AnythingOfType("*v1alpha1.Package"),
						mock.Anything,
					).Run(func(args mock.Arguments) {
						arg := args.Get(2).(*corev1alpha1.Package)
						*arg = pkg
					}).Return(nil)
				}

				c.StatusMock.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.HostedClusterPackage{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					hcpkg := args.Get(1).(*corev1alpha1.HostedClusterPackage)

					// Validate counts
					assert.Equal(t, int32(1), hcpkg.Status.ObservedGeneration)
					assert.Equal(t, int32(5), hcpkg.Status.TotalPackages)
					assert.Equal(t, int32(5), hcpkg.Status.AvailablePackages)
					assert.Equal(t, int32(0), hcpkg.Status.ProgressedPackages)
					assert.Equal(t, int32(5), hcpkg.Status.UpdatedPackages)

					// Validate conditions
					require.Len(t, hcpkg.Status.Conditions, 2)

					availableCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageAvailable)
					require.NotNil(t, availableCond)
					assert.Equal(t, metav1.ConditionTrue, availableCond.Status)
					assert.Equal(t, int64(1), availableCond.ObservedGeneration)
					assert.Equal(t, "5/5 packages available.", availableCond.Message)

					progressingCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageProgressing)
					require.NotNil(t, progressingCond)
					assert.Equal(t, metav1.ConditionFalse, progressingCond.Status)
					assert.Equal(t, int64(1), progressingCond.ObservedGeneration)
					assert.Equal(t, "0/5 packages progressed.", progressingCond.Message)
				}).Return(nil)
			},
			expectedResult: ctrl.Result{},
		},
		{
			name: "All 5 packages unavailable and not progressed",
			setupClient: func(c *testutil.CtrlClient) {
				hcpkg := &corev1alpha1.HostedClusterPackage{
					ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg", Generation: 1},
					Spec: corev1alpha1.HostedClusterPackageSpec{
						Strategy: corev1alpha1.HostedClusterPackageStrategy{
							RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
								MaxUnavailable: 1,
							},
						},
						Template: corev1alpha1.PackageTemplateSpec{
							Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
						},
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

				hostedClusters := makeHostedClusters()
				c.On("List",
					mock.Anything,
					mock.AnythingOfType("*v1beta1.HostedClusterList"),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(1).(*hypershiftv1beta1.HostedClusterList)
					*arg = hypershiftv1beta1.HostedClusterList{Items: hostedClusters}
				}).Return(nil)

				for i := range 5 {
					pkg := makePackage(i, false, false)
					c.On("Get",
						mock.Anything,
						types.NamespacedName{Name: "test-hcpkg", Namespace: fmt.Sprintf("default-hc-%d", i)},
						mock.AnythingOfType("*v1alpha1.Package"),
						mock.Anything,
					).Run(func(args mock.Arguments) {
						arg := args.Get(2).(*corev1alpha1.Package)
						*arg = pkg
					}).Return(nil)
				}

				c.StatusMock.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.HostedClusterPackage{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					hcpkg := args.Get(1).(*corev1alpha1.HostedClusterPackage)

					// Validate counts
					assert.Equal(t, int32(1), hcpkg.Status.ObservedGeneration)
					assert.Equal(t, int32(5), hcpkg.Status.TotalPackages)
					assert.Equal(t, int32(0), hcpkg.Status.AvailablePackages)
					assert.Equal(t, int32(0), hcpkg.Status.ProgressedPackages)
					assert.Equal(t, int32(5), hcpkg.Status.UpdatedPackages)

					// Validate conditions
					require.Len(t, hcpkg.Status.Conditions, 2)

					availableCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageAvailable)
					require.NotNil(t, availableCond)
					assert.Equal(t, metav1.ConditionFalse, availableCond.Status)
					assert.Equal(t, int64(1), availableCond.ObservedGeneration)
					assert.Equal(t, "0/5 packages available.", availableCond.Message)

					progressingCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageProgressing)
					require.NotNil(t, progressingCond)
					assert.Equal(t, metav1.ConditionFalse, progressingCond.Status)
					assert.Equal(t, int64(1), progressingCond.ObservedGeneration)
					assert.Equal(t, "0/5 packages progressed.", progressingCond.Message)
				}).Return(nil)
			},
			expectedResult: ctrl.Result{},
		},
		{
			name: "3 available/progressed, 2 unavailable/not progressed",
			setupClient: func(c *testutil.CtrlClient) {
				hcpkg := &corev1alpha1.HostedClusterPackage{
					ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg", Generation: 1},
					Spec: corev1alpha1.HostedClusterPackageSpec{
						Strategy: corev1alpha1.HostedClusterPackageStrategy{
							RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
								MaxUnavailable: 2,
							},
						},
						Template: corev1alpha1.PackageTemplateSpec{
							Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
						},
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

				hostedClusters := makeHostedClusters()
				c.On("List",
					mock.Anything,
					mock.AnythingOfType("*v1beta1.HostedClusterList"),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(1).(*hypershiftv1beta1.HostedClusterList)
					*arg = hypershiftv1beta1.HostedClusterList{Items: hostedClusters}
				}).Return(nil)

				for i := range 5 {
					available := i < 3
					progressed := i < 3
					pkg := makePackage(i, available, progressed)
					c.On("Get",
						mock.Anything,
						types.NamespacedName{Name: "test-hcpkg", Namespace: fmt.Sprintf("default-hc-%d", i)},
						mock.AnythingOfType("*v1alpha1.Package"),
						mock.Anything,
					).Run(func(args mock.Arguments) {
						arg := args.Get(2).(*corev1alpha1.Package)
						*arg = pkg
					}).Return(nil)
				}

				c.StatusMock.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.HostedClusterPackage{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					hcpkg := args.Get(1).(*corev1alpha1.HostedClusterPackage)

					// Validate counts
					assert.Equal(t, int32(1), hcpkg.Status.ObservedGeneration)
					assert.Equal(t, int32(5), hcpkg.Status.TotalPackages)
					assert.Equal(t, int32(3), hcpkg.Status.AvailablePackages)
					assert.Equal(t, int32(3), hcpkg.Status.ProgressedPackages)
					assert.Equal(t, int32(5), hcpkg.Status.UpdatedPackages)

					// Validate conditions
					require.Len(t, hcpkg.Status.Conditions, 2)

					availableCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageAvailable)
					require.NotNil(t, availableCond)
					assert.Equal(t, metav1.ConditionTrue, availableCond.Status)
					assert.Equal(t, int64(1), availableCond.ObservedGeneration)
					assert.Equal(t, "3/5 packages available.", availableCond.Message)

					progressingCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageProgressing)
					require.NotNil(t, progressingCond)
					assert.Equal(t, metav1.ConditionFalse, progressingCond.Status)
					assert.Equal(t, int64(1), progressingCond.ObservedGeneration)
					assert.Equal(t, "3/5 packages progressed.", progressingCond.Message)
				}).Return(nil)
			},
			expectedResult: ctrl.Result{},
		},
		{
			name: "All available, mixed progression (3 progressed, 2 not)",
			setupClient: func(c *testutil.CtrlClient) {
				hcpkg := &corev1alpha1.HostedClusterPackage{
					ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg", Generation: 1},
					Spec: corev1alpha1.HostedClusterPackageSpec{
						Strategy: corev1alpha1.HostedClusterPackageStrategy{
							RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
								MaxUnavailable: 1,
							},
						},
						Template: corev1alpha1.PackageTemplateSpec{
							Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
						},
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

				hostedClusters := makeHostedClusters()
				c.On("List",
					mock.Anything,
					mock.AnythingOfType("*v1beta1.HostedClusterList"),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(1).(*hypershiftv1beta1.HostedClusterList)
					*arg = hypershiftv1beta1.HostedClusterList{Items: hostedClusters}
				}).Return(nil)

				for i := range 5 {
					progressed := i < 3
					pkg := makePackage(i, true, progressed)
					c.On("Get",
						mock.Anything,
						types.NamespacedName{Name: "test-hcpkg", Namespace: fmt.Sprintf("default-hc-%d", i)},
						mock.AnythingOfType("*v1alpha1.Package"),
						mock.Anything,
					).Run(func(args mock.Arguments) {
						arg := args.Get(2).(*corev1alpha1.Package)
						*arg = pkg
					}).Return(nil)
				}

				c.StatusMock.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.HostedClusterPackage{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					hcpkg := args.Get(1).(*corev1alpha1.HostedClusterPackage)

					// Validate counts
					assert.Equal(t, int32(1), hcpkg.Status.ObservedGeneration)
					assert.Equal(t, int32(5), hcpkg.Status.TotalPackages)
					assert.Equal(t, int32(5), hcpkg.Status.AvailablePackages)
					assert.Equal(t, int32(3), hcpkg.Status.ProgressedPackages)
					assert.Equal(t, int32(5), hcpkg.Status.UpdatedPackages)

					// Validate conditions
					require.Len(t, hcpkg.Status.Conditions, 2)

					availableCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageAvailable)
					require.NotNil(t, availableCond)
					assert.Equal(t, metav1.ConditionTrue, availableCond.Status)
					assert.Equal(t, int64(1), availableCond.ObservedGeneration)
					assert.Equal(t, "5/5 packages available.", availableCond.Message)

					progressingCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageProgressing)
					require.NotNil(t, progressingCond)
					assert.Equal(t, metav1.ConditionFalse, progressingCond.Status)
					assert.Equal(t, int64(1), progressingCond.ObservedGeneration)
					assert.Equal(t, "3/5 packages progressed.", progressingCond.Message)
				}).Return(nil)
			},
			expectedResult: ctrl.Result{},
		},
		{
			name: "Mixed availability (2 available, 3 unavailable), all progressed",
			setupClient: func(c *testutil.CtrlClient) {
				hcpkg := &corev1alpha1.HostedClusterPackage{
					ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg", Generation: 1},
					Spec: corev1alpha1.HostedClusterPackageSpec{
						Strategy: corev1alpha1.HostedClusterPackageStrategy{
							RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
								MaxUnavailable: 3,
							},
						},
						Template: corev1alpha1.PackageTemplateSpec{
							Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
						},
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

				hostedClusters := makeHostedClusters()
				c.On("List",
					mock.Anything,
					mock.AnythingOfType("*v1beta1.HostedClusterList"),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(1).(*hypershiftv1beta1.HostedClusterList)
					*arg = hypershiftv1beta1.HostedClusterList{Items: hostedClusters}
				}).Return(nil)

				for i := range 5 {
					available := i < 2
					pkg := makePackage(i, available, true)
					c.On("Get",
						mock.Anything,
						types.NamespacedName{Name: "test-hcpkg", Namespace: fmt.Sprintf("default-hc-%d", i)},
						mock.AnythingOfType("*v1alpha1.Package"),
						mock.Anything,
					).Run(func(args mock.Arguments) {
						arg := args.Get(2).(*corev1alpha1.Package)
						*arg = pkg
					}).Return(nil)
				}

				c.StatusMock.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.HostedClusterPackage{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					hcpkg := args.Get(1).(*corev1alpha1.HostedClusterPackage)

					// Validate counts
					assert.Equal(t, int32(1), hcpkg.Status.ObservedGeneration)
					assert.Equal(t, int32(5), hcpkg.Status.TotalPackages)
					assert.Equal(t, int32(2), hcpkg.Status.AvailablePackages)
					assert.Equal(t, int32(5), hcpkg.Status.ProgressedPackages)
					assert.Equal(t, int32(5), hcpkg.Status.UpdatedPackages)

					// Validate conditions
					require.Len(t, hcpkg.Status.Conditions, 2)

					availableCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageAvailable)
					require.NotNil(t, availableCond)
					assert.Equal(t, metav1.ConditionTrue, availableCond.Status)
					assert.Equal(t, int64(1), availableCond.ObservedGeneration)
					assert.Equal(t, "2/5 packages available.", availableCond.Message)

					progressingCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageProgressing)
					require.NotNil(t, progressingCond)
					assert.Equal(t, metav1.ConditionTrue, progressingCond.Status)
					assert.Equal(t, int64(1), progressingCond.ObservedGeneration)
					assert.Equal(t, "5/5 packages progressed.", progressingCond.Message)
				}).Return(nil)
			},
			expectedResult: ctrl.Result{},
		},
		{
			name: "Complex mix: 2 available+progressed, 1 available+not progressed, 2 unavailable+not progressed",
			setupClient: func(c *testutil.CtrlClient) {
				hcpkg := &corev1alpha1.HostedClusterPackage{
					ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg", Generation: 1},
					Spec: corev1alpha1.HostedClusterPackageSpec{
						Strategy: corev1alpha1.HostedClusterPackageStrategy{
							RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
								MaxUnavailable: 2,
							},
						},
						Template: corev1alpha1.PackageTemplateSpec{
							Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
						},
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

				hostedClusters := makeHostedClusters()
				c.On("List",
					mock.Anything,
					mock.AnythingOfType("*v1beta1.HostedClusterList"),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(1).(*hypershiftv1beta1.HostedClusterList)
					*arg = hypershiftv1beta1.HostedClusterList{Items: hostedClusters}
				}).Return(nil)

				packageStates := []struct{ available, progressed bool }{
					{true, true},   // pkg 0
					{true, true},   // pkg 1
					{true, false},  // pkg 2
					{false, false}, // pkg 3
					{false, false}, // pkg 4
				}

				for i := range 5 {
					pkg := makePackage(i, packageStates[i].available, packageStates[i].progressed)
					c.On("Get",
						mock.Anything,
						types.NamespacedName{Name: "test-hcpkg", Namespace: fmt.Sprintf("default-hc-%d", i)},
						mock.AnythingOfType("*v1alpha1.Package"),
						mock.Anything,
					).Run(func(args mock.Arguments) {
						arg := args.Get(2).(*corev1alpha1.Package)
						*arg = pkg
					}).Return(nil)
				}

				c.StatusMock.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.HostedClusterPackage{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					hcpkg := args.Get(1).(*corev1alpha1.HostedClusterPackage)

					// Validate counts
					assert.Equal(t, int32(1), hcpkg.Status.ObservedGeneration)
					assert.Equal(t, int32(5), hcpkg.Status.TotalPackages)
					assert.Equal(t, int32(3), hcpkg.Status.AvailablePackages)
					assert.Equal(t, int32(2), hcpkg.Status.ProgressedPackages)
					assert.Equal(t, int32(5), hcpkg.Status.UpdatedPackages)

					// Validate conditions
					require.Len(t, hcpkg.Status.Conditions, 2)

					availableCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageAvailable)
					require.NotNil(t, availableCond)
					assert.Equal(t, metav1.ConditionTrue, availableCond.Status)
					assert.Equal(t, int64(1), availableCond.ObservedGeneration)
					assert.Equal(t, "3/5 packages available.", availableCond.Message)

					progressingCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageProgressing)
					require.NotNil(t, progressingCond)
					assert.Equal(t, metav1.ConditionFalse, progressingCond.Status)
					assert.Equal(t, int64(1), progressingCond.ObservedGeneration)
					assert.Equal(t, "2/5 packages progressed.", progressingCond.Message)
				}).Return(nil)
			},
			expectedResult: ctrl.Result{},
		},
		{
			name: "Boundary: 1 available+progressed, 4 unavailable+not progressed (exceeds maxUnavailable)",
			setupClient: func(c *testutil.CtrlClient) {
				hcpkg := &corev1alpha1.HostedClusterPackage{
					ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg", Generation: 1},
					Spec: corev1alpha1.HostedClusterPackageSpec{
						Strategy: corev1alpha1.HostedClusterPackageStrategy{
							RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
								MaxUnavailable: 2,
							},
						},
						Template: corev1alpha1.PackageTemplateSpec{
							Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
						},
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

				hostedClusters := makeHostedClusters()
				c.On("List",
					mock.Anything,
					mock.AnythingOfType("*v1beta1.HostedClusterList"),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(1).(*hypershiftv1beta1.HostedClusterList)
					*arg = hypershiftv1beta1.HostedClusterList{Items: hostedClusters}
				}).Return(nil)

				for i := range 5 {
					available := i == 0
					progressed := i == 0
					pkg := makePackage(i, available, progressed)
					c.On("Get",
						mock.Anything,
						types.NamespacedName{Name: "test-hcpkg", Namespace: fmt.Sprintf("default-hc-%d", i)},
						mock.AnythingOfType("*v1alpha1.Package"),
						mock.Anything,
					).Run(func(args mock.Arguments) {
						arg := args.Get(2).(*corev1alpha1.Package)
						*arg = pkg
					}).Return(nil)
				}

				c.StatusMock.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.HostedClusterPackage{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					hcpkg := args.Get(1).(*corev1alpha1.HostedClusterPackage)

					// Validate counts
					assert.Equal(t, int32(1), hcpkg.Status.ObservedGeneration)
					assert.Equal(t, int32(5), hcpkg.Status.TotalPackages)
					assert.Equal(t, int32(1), hcpkg.Status.AvailablePackages)
					assert.Equal(t, int32(1), hcpkg.Status.ProgressedPackages)
					assert.Equal(t, int32(5), hcpkg.Status.UpdatedPackages)

					// Validate conditions
					require.Len(t, hcpkg.Status.Conditions, 2)

					availableCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageAvailable)
					require.NotNil(t, availableCond)
					assert.Equal(t, metav1.ConditionFalse, availableCond.Status)
					assert.Equal(t, int64(1), availableCond.ObservedGeneration)
					assert.Equal(t, "1/5 packages available.", availableCond.Message)

					progressingCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageProgressing)
					require.NotNil(t, progressingCond)
					assert.Equal(t, metav1.ConditionFalse, progressingCond.Status)
					assert.Equal(t, int64(1), progressingCond.ObservedGeneration)
					assert.Equal(t, "1/5 packages progressed.", progressingCond.Message)
				}).Return(nil)
			},
			expectedResult: ctrl.Result{},
		},
		{
			name: "Instant strategy with mixed states",
			setupClient: func(c *testutil.CtrlClient) {
				hcpkg := &corev1alpha1.HostedClusterPackage{
					ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg", Generation: 1},
					Spec: corev1alpha1.HostedClusterPackageSpec{
						Strategy: corev1alpha1.HostedClusterPackageStrategy{
							Instant: &corev1alpha1.HostedClusterPackageStrategyInstant{},
						},
						Template: corev1alpha1.PackageTemplateSpec{
							Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
						},
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

				hostedClusters := makeHostedClusters()
				c.On("List",
					mock.Anything,
					mock.AnythingOfType("*v1beta1.HostedClusterList"),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(1).(*hypershiftv1beta1.HostedClusterList)
					*arg = hypershiftv1beta1.HostedClusterList{Items: hostedClusters}
				}).Return(nil)

				packageStates := []struct{ available, progressed bool }{
					{true, true},
					{true, false},
					{false, true},
					{false, false},
					{true, true},
				}

				for i := range 5 {
					pkg := makePackage(i, packageStates[i].available, packageStates[i].progressed)
					c.On("Get",
						mock.Anything,
						types.NamespacedName{Name: "test-hcpkg", Namespace: fmt.Sprintf("default-hc-%d", i)},
						mock.AnythingOfType("*v1alpha1.Package"),
						mock.Anything,
					).Run(func(args mock.Arguments) {
						arg := args.Get(2).(*corev1alpha1.Package)
						*arg = pkg
					}).Return(nil)
				}

				c.StatusMock.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.HostedClusterPackage{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					hcpkg := args.Get(1).(*corev1alpha1.HostedClusterPackage)

					// Validate counts
					assert.Equal(t, int32(1), hcpkg.Status.ObservedGeneration)
					assert.Equal(t, int32(5), hcpkg.Status.TotalPackages)
					assert.Equal(t, int32(3), hcpkg.Status.AvailablePackages)
					assert.Equal(t, int32(3), hcpkg.Status.ProgressedPackages)
					assert.Equal(t, int32(5), hcpkg.Status.UpdatedPackages)

					// Validate conditions
					require.Len(t, hcpkg.Status.Conditions, 2)

					availableCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageAvailable)
					require.NotNil(t, availableCond)
					assert.Equal(t, metav1.ConditionFalse, availableCond.Status)
					assert.Equal(t, int64(1), availableCond.ObservedGeneration)
					assert.Equal(t, "3/5 packages available.", availableCond.Message)

					progressingCond := meta.FindStatusCondition(hcpkg.Status.Conditions, corev1alpha1.HostedClusterPackageProgressing)
					require.NotNil(t, progressingCond)
					assert.Equal(t, metav1.ConditionFalse, progressingCond.Status)
					assert.Equal(t, int64(1), progressingCond.ObservedGeneration)
					assert.Equal(t, "3/5 packages progressed.", progressingCond.Message)
				}).Return(nil)
			},
			expectedResult: ctrl.Result{},
		},
		{
			name: "Some HostedClusters not available yet",
			setupClient: func(c *testutil.CtrlClient) {
				hcpkg := &corev1alpha1.HostedClusterPackage{
					ObjectMeta: metav1.ObjectMeta{Name: "test-hcpkg", Generation: 1},
					Spec: corev1alpha1.HostedClusterPackageSpec{
						Strategy: corev1alpha1.HostedClusterPackageStrategy{
							RollingUpgrade: &corev1alpha1.HostedClusterPackageStrategyRollingUpgrade{
								MaxUnavailable: 2,
							},
						},
						Template: corev1alpha1.PackageTemplateSpec{
							Spec: corev1alpha1.PackageSpec{Image: "test-image:v1"},
						},
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

				hostedClusters := make([]hypershiftv1beta1.HostedCluster, 5)
				for i := range 5 {
					hostedClusters[i] = hypershiftv1beta1.HostedCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("hc-%d", i),
							Namespace: "default",
							UID:       types.UID(fmt.Sprintf("hc-uid-%d", i)),
						},
						Status: hypershiftv1beta1.HostedClusterStatus{
							Conditions: []metav1.Condition{},
						},
					}
					// Only first 3 HostedClusters are available
					if i < 3 {
						hostedClusters[i].Status.Conditions = []metav1.Condition{
							{
								Type:   hypershiftv1beta1.HostedClusterAvailable,
								Status: metav1.ConditionTrue,
							},
						}
					}
				}

				c.On("List",
					mock.Anything,
					mock.AnythingOfType("*v1beta1.HostedClusterList"),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(1).(*hypershiftv1beta1.HostedClusterList)
					*arg = hypershiftv1beta1.HostedClusterList{Items: hostedClusters}
				}).Return(nil)

				// Only the available HostedClusters have packages
				for i := range 3 {
					pkg := makePackage(i, true, true)
					c.On("Get",
						mock.Anything,
						types.NamespacedName{Name: "test-hcpkg", Namespace: fmt.Sprintf("default-hc-%d", i)},
						mock.AnythingOfType("*v1alpha1.Package"),
						mock.Anything,
					).Run(func(args mock.Arguments) {
						arg := args.Get(2).(*corev1alpha1.Package)
						*arg = pkg
					}).Return(nil)
				}

				// Last 2 HostedClusters don't have packages yet
				for i := 3; i < 5; i++ {
					c.On("Get",
						mock.Anything,
						types.NamespacedName{Name: "test-hcpkg", Namespace: fmt.Sprintf("default-hc-%d", i)},
						mock.AnythingOfType("*v1alpha1.Package"),
						mock.Anything,
					).Return(errors.NewNotFound(schema.GroupResource{}, "test-package"))
				}

				// Mock creating packages for the missing HostedClusters
				c.On("Create",
					mock.Anything,
					mock.AnythingOfType("*v1alpha1.Package"),
					mock.Anything,
				).Return(nil)

				c.StatusMock.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.HostedClusterPackage{}),
					mock.Anything,
				).Return(nil)
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

	pkg, err := controller.constructPackage(hostedClusterPackage, hostedCluster)

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

// makeHostedClusters creates 5 HostedCluster test objects.
func makeHostedClusters() []hypershiftv1beta1.HostedCluster {
	n := 5
	clusters := make([]hypershiftv1beta1.HostedCluster, 0, n)
	for i := range n {
		clusters = append(clusters, hypershiftv1beta1.HostedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("hc-%d", i),
				Namespace: "default",
				UID:       types.UID(fmt.Sprintf("hc-uid-%d", i)),
			},
			Status: hypershiftv1beta1.HostedClusterStatus{
				Conditions: []metav1.Condition{
					{
						Type:   hypershiftv1beta1.HostedClusterAvailable,
						Status: metav1.ConditionTrue,
					},
				},
			},
		})
	}
	return clusters
}

// makePackage creates a Package test object with specified availability and progression states.
func makePackage(id int, available, progressed bool) corev1alpha1.Package {
	pkg := corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-hcpkg",
			Namespace:  fmt.Sprintf("default-hc-%d", id),
			UID:        types.UID(fmt.Sprintf("pkg-uid-%d", id)),
			Generation: 1,
		},
		Spec: corev1alpha1.PackageSpec{
			Image: "test-image:v1",
		},
		Status: corev1alpha1.PackageStatus{
			Conditions: []metav1.Condition{},
		},
	}

	if available {
		pkg.Status.Conditions = append(pkg.Status.Conditions, metav1.Condition{
			Type:               corev1alpha1.PackageAvailable,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: 1,
		})
	} else {
		pkg.Status.Conditions = append(pkg.Status.Conditions, metav1.Condition{
			Type:               corev1alpha1.PackageAvailable,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: 1,
		})
	}

	if progressed {
		pkg.Status.Conditions = append(pkg.Status.Conditions,
			metav1.Condition{
				Type:               corev1alpha1.PackageUnpacked,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 1,
			},
			metav1.Condition{
				Type:               corev1alpha1.PackageProgressing,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: 1,
			},
		)
	} else {
		pkg.Status.Conditions = append(pkg.Status.Conditions,
			metav1.Condition{
				Type:               corev1alpha1.PackageUnpacked,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: 1,
			},
			metav1.Condition{
				Type:               corev1alpha1.PackageProgressing,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 1,
			},
		)
	}

	return pkg
}
