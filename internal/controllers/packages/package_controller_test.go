package packages

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
)

func TestGenericPackageController(t *testing.T) {
	t.Run("Package not found", func(t *testing.T) {
		pkoNamespace := "package-operator-system"
		pkgName, pkgNamespace := "foo", "foo-ns"

		controller, testClient, _, _ := newControllerAndMocks(pkoNamespace)
		testClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errWithStatusError{errStatusReason: metav1.StatusReasonNotFound})
		res, err := controller.Reconcile(context.TODO(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pkgName, Namespace: pkgNamespace}})
		testClient.AssertCalled(t, "Get", mock.Anything, types.NamespacedName{Name: pkgName, Namespace: pkgNamespace}, mock.Anything, mock.Anything)
		require.NoError(t, err)
		require.Equal(t, ctrl.Result{}, res)
	})

	testCases := []struct {
		name                                string
		finalizersOnPkg                     []string
		jobReconcilerErr                    error
		objectDeploymentStatusReconcilerErr error
		deletionTimestamp                   *metav1.Time
	}{
		{
			name:                                "package found with no finalizers",
			finalizersOnPkg:                     []string{},
			jobReconcilerErr:                    nil,
			objectDeploymentStatusReconcilerErr: nil,
		},
		{
			name:                                "package found with job reconciler err-ing out",
			finalizersOnPkg:                     getPackageFinalizerNames(),
			jobReconcilerErr:                    errWithStatusError{errMsg: "job reconciliation failed"},
			objectDeploymentStatusReconcilerErr: nil,
		},
		{
			name:                                "package found with objectDeployment reconciler err-ing out",
			finalizersOnPkg:                     []string{},
			jobReconcilerErr:                    nil,
			objectDeploymentStatusReconcilerErr: errWithStatusError{errMsg: "objectDeployment reconciliation failed"},
		},
		{
			name:                                "package found with no finalizers and both objectDeployment and job reconciler err-ing out",
			finalizersOnPkg:                     getPackageFinalizerNames(),
			jobReconcilerErr:                    errWithStatusError{errMsg: "job reconciliation failed"},
			objectDeploymentStatusReconcilerErr: errWithStatusError{errMsg: "objectDeployment reconciliation failed"},
		},
		{
			name:                                "package found with no finalizers and a deletion timestamp",
			finalizersOnPkg:                     []string{},
			jobReconcilerErr:                    nil,
			objectDeploymentStatusReconcilerErr: nil,
			deletionTimestamp:                   &metav1.Time{Time: time.Now()},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			pkoNamespace := "package-operator-system"
			pkgName, pkgNamespace := "foo", "foo-ns"

			controller, testClient, or, jr := newControllerAndMocks(pkoNamespace)
			foundPkg := &corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:              pkgName,
					Namespace:         pkgNamespace,
					Finalizers:        testCase.finalizersOnPkg,
					DeletionTimestamp: testCase.deletionTimestamp,
				},
			}
			testClient.On("Get", mock.Anything, mock.Anything, &corev1alpha1.Package{}, mock.Anything).Run(func(args mock.Arguments) {
				destination := args.Get(2).(*corev1alpha1.Package)
				foundPkg.DeepCopyInto(destination)
			}).Return(nil)

			testClient.On("Get", mock.Anything, mock.Anything, &batchv1.Job{}, mock.Anything).Return(nil)
			testClient.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			testClient.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe()
			or.On("Reconcile", mock.Anything, mock.Anything).Return(ctrl.Result{}, testCase.objectDeploymentStatusReconcilerErr)
			jr.On("Reconcile", mock.Anything, mock.Anything).Return(ctrl.Result{}, testCase.jobReconcilerErr)
			testClient.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe()

			res, recErr := controller.Reconcile(context.TODO(), ctrl.Request{NamespacedName: types.NamespacedName{Name: pkgName, Namespace: pkgNamespace}})

			testClient.AssertCalled(t, "Get", mock.Anything, types.NamespacedName{Name: pkgName, Namespace: pkgNamespace}, mock.Anything, mock.Anything)

			if !sortInsensitiveStringSlicesMatch(getPackageFinalizerNames(), testCase.finalizersOnPkg) || !foundPkg.DeletionTimestamp.IsZero() {
				testClient.AssertCalled(t, "Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			} else {
				testClient.AssertNotCalled(t, "Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}

			if !foundPkg.DeletionTimestamp.IsZero() {
				jobName, jobNamespace := fmt.Sprintf("job-%s", pkgName), pkoNamespace
				testClient.AssertCalled(t, "Get", mock.Anything, types.NamespacedName{Name: jobName, Namespace: jobNamespace}, mock.Anything, mock.Anything)
				testClient.AssertCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
				or.AssertNotCalled(t, "Reconcile", mock.Anything, mock.Anything)
				jr.AssertNotCalled(t, "Reconcile", mock.Anything, mock.Anything)
				testClient.StatusMock.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				return
			}

			require.Equal(t, ctrl.Result{}, res)

			if testCase.objectDeploymentStatusReconcilerErr != nil {
				require.Equal(t, testCase.objectDeploymentStatusReconcilerErr, recErr)
				or.AssertCalled(t, "Reconcile", mock.Anything, mock.Anything)
				jr.AssertNotCalled(t, "Reconcile", mock.Anything, mock.Anything)
				testClient.StatusMock.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				return
			}
			if testCase.jobReconcilerErr != nil {
				require.Equal(t, testCase.jobReconcilerErr, recErr)
				or.AssertCalled(t, "Reconcile", mock.Anything, mock.Anything)
				jr.AssertCalled(t, "Reconcile", mock.Anything, mock.Anything)
				testClient.StatusMock.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				return
			}
			require.NoError(t, recErr)
			or.AssertCalled(t, "Reconcile", mock.Anything, mock.Anything)
			jr.AssertCalled(t, "Reconcile", mock.Anything, mock.Anything)
			testClient.StatusMock.AssertCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}

}

func newControllerAndMocks(pkoNamespace string) (*GenericPackageController, *testutil.CtrlClient, *objectDeploymentStatusReconcilerMock, *jobReconcilerMock) {
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	c := testutil.NewClient()

	controller := &GenericPackageController{
		newPackage:          newGenericPackage,
		newObjectDeployment: newGenericObjectDeployment,
		client:              c,
		log:                 ctrl.Log.WithName("controllers"),
		scheme:              scheme,
		pkoNamespace:        pkoNamespace,
	}
	or := &objectDeploymentStatusReconcilerMock{}
	jr := &jobReconcilerMock{}
	controller.reconciler = []reconciler{
		or,
		jr,
	}
	return controller, c, or, jr
}
