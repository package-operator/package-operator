package integration

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// Simple Setup and Teardown test.
func TestPackage_creationAndDeletion(t *testing.T) {
	packageGen := func(packageName string, packageImage string) *corev1alpha1.Package {
		return &corev1alpha1.Package{
			ObjectMeta: metav1.ObjectMeta{
				Name:      packageName,
				Namespace: "default",
			},
			Spec: corev1alpha1.PackageSpec{
				Image: packageImage,
			},
		}
	}

	testCases := []struct {
		packageName                     string
		packageImage                    string
		jobShouldBeCreated              bool
		objectDeploymentShouldBeCreated bool
		becomesAvailable                bool
		expectedPackageConditionType    string
		expectedPackageConditionStatus  metav1.ConditionStatus
	}{
		{
			packageName:                     "foo",
			packageImage:                    SuccessTestPackageImage,
			objectDeploymentShouldBeCreated: true,
			becomesAvailable:                true,
			expectedPackageConditionType:    corev1alpha1.PackageUnpacked,
			expectedPackageConditionStatus:  metav1.ConditionTrue,
		},
		{
			packageName:                     "bar",
			packageImage:                    "quay.io/non-existent/image:test",
			objectDeploymentShouldBeCreated: false,
			expectedPackageConditionType:    corev1alpha1.PackageUnpacked,
			expectedPackageConditionStatus:  metav1.ConditionFalse,
		},
	}

	ctx := logr.NewContext(context.Background(), testr.New(t))

	for _, testCase := range testCases {
		pkg := packageGen(testCase.packageName, testCase.packageImage)
		require.NoError(t, Client.Create(ctx, pkg))
		cleanupOnSuccess(ctx, t, pkg)

		// Wait for Unpacked condition to be reported.
		require.NoError(t, Waiter.WaitForCondition(
			ctx, pkg, testCase.expectedPackageConditionType, testCase.expectedPackageConditionStatus))

		if testCase.objectDeploymentShouldBeCreated {
			expectedObjectDeployment := &corev1alpha1.ObjectDeployment{
				ObjectMeta: metav1.ObjectMeta{}}

			require.NoError(t, Client.Get(ctx, client.ObjectKey{
				Name: pkg.Name, Namespace: "default",
			}, expectedObjectDeployment))
		}

		if testCase.becomesAvailable {
			require.NoError(t,
				Waiter.WaitForCondition(ctx, pkg, corev1alpha1.PackageAvailable, metav1.ConditionTrue))
		}
	}
}
