package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// Simple Setup and Teardown test.
func TestPackage_creationAndDeletion(t *testing.T) {
	packageGen := func(packageName string, packageNamespace string, packageImage string) *corev1alpha1.Package {
		return &corev1alpha1.Package{
			ObjectMeta: metav1.ObjectMeta{
				Name:      packageName,
				Namespace: packageNamespace,
			},
			Spec: corev1alpha1.PackageSpec{
				Image: packageImage,
			},
		}
	}

	testCases := []struct {
		packageName                     string
		packageNamespace                string
		packageImage                    string
		jobShouldBeCreated              bool
		objectDeploymentShouldBeCreated bool
		expectedPackageConditionType    string
		cleanupOnExit                   bool
		expectedPackageConditionStatus  metav1.ConditionStatus
	}{
		{
			packageName:                     "foo",
			packageNamespace:                "foo-ns",
			packageImage:                    "quay.io/nschiede/foo:test", // this gets loaded as a part of integration setup defined in magefile.go
			jobShouldBeCreated:              true,
			objectDeploymentShouldBeCreated: true,
			cleanupOnExit:                   true,
			expectedPackageConditionType:    corev1alpha1.PackageUnpacked,
			expectedPackageConditionStatus:  metav1.ConditionTrue,
		},
		{
			packageName:                     "bar",
			packageNamespace:                "bar-ns",
			packageImage:                    "quay.io/non-existent/image:test",
			jobShouldBeCreated:              true,
			objectDeploymentShouldBeCreated: false,
			cleanupOnExit:                   true,
			expectedPackageConditionType:    corev1alpha1.PackageUnpacked,
			expectedPackageConditionStatus:  metav1.ConditionFalse,
		},
	}

	ctx := context.TODO()
	pkoNamespace, err := findPackageOperatorNamespace(context.TODO())
	require.NoError(t, err)
	defer func() {
		for _, testCase := range testCases {
			if testCase.cleanupOnExit {
				err := Client.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testCase.packageNamespace}})
				require.NoError(t, client.IgnoreNotFound(err))

				job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "job-" + testCase.packageName, Namespace: pkoNamespace}}
				err = Client.Delete(ctx, job)
				require.NoError(t, client.IgnoreNotFound(err))
			}
		}
	}()

	for _, testCase := range testCases {
		pkg := packageGen(testCase.packageName, testCase.packageNamespace, testCase.packageImage)
		err := Client.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testCase.packageNamespace}})
		err = client.IgnoreAlreadyExists(err)
		require.NoError(t, err)

		err = Client.Create(ctx, pkg)
		require.NoError(t, err)

		expectedJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "job-" + pkg.Name, Namespace: pkoNamespace}}
		if testCase.jobShouldBeCreated {
			err = Waiter.WaitForObject(ctx, expectedJob, "waiting for the job to be created", func(obj client.Object) (done bool, err error) { return true, nil })
			require.NoError(t, err)
		}

		expectedObjectDeployment := &corev1alpha1.ObjectDeployment{ObjectMeta: metav1.ObjectMeta{Name: pkg.Name, Namespace: pkg.Namespace}}
		if testCase.objectDeploymentShouldBeCreated {
			err = Waiter.WaitForObject(ctx, expectedObjectDeployment, "waiting for the objectDeployment to be created", func(obj client.Object) (done bool, err error) { return true, nil })
			require.NoError(t, err)
		}

		err = Waiter.WaitForCondition(ctx, pkg, testCase.expectedPackageConditionType, testCase.expectedPackageConditionStatus)
		require.NoError(t, err)

		// test deletion
		err = Client.Delete(ctx, pkg)
		require.NoError(t, err)

		err = Waiter.WaitToBeGone(ctx, pkg, func(obj client.Object) (done bool, err error) { return false, nil })
		require.NoError(t, err)

		err = Waiter.WaitToBeGone(ctx, expectedJob, func(obj client.Object) (done bool, err error) { return false, nil })
		require.NoError(t, err)

		err = Waiter.WaitToBeGone(ctx, expectedObjectDeployment, func(obj client.Object) (done bool, err error) { return false, nil })
		require.NoError(t, err)
	}
}

func TestClusterPackage_creationAndDeletion(t *testing.T) {
	clusterPackageGen := func(packageName string, packageImage string) *corev1alpha1.ClusterPackage {
		return &corev1alpha1.ClusterPackage{
			ObjectMeta: metav1.ObjectMeta{
				Name: packageName,
			},
			Spec: corev1alpha1.PackageSpec{
				Image: packageImage,
			},
		}
	}

	testCases := []struct {
		packageName                            string
		packageImage                           string
		jobShouldBeCreated                     bool
		clusterObjectDeploymentShouldBeCreated bool
		expectedPackageConditionType           string
		cleanupOnExit                          bool
		expectedPackageConditionStatus         metav1.ConditionStatus
	}{
		{
			packageName:                            "foo",
			packageImage:                           "quay.io/nschiede/foo:test", // this gets loaded as a part of integration setup defined in magefile.go
			jobShouldBeCreated:                     true,
			clusterObjectDeploymentShouldBeCreated: true,
			cleanupOnExit:                          true,
			expectedPackageConditionType:           corev1alpha1.PackageUnpacked,
			expectedPackageConditionStatus:         metav1.ConditionTrue,
		},
		{
			packageName:                            "bar",
			packageImage:                           "quay.io/non-existent/image:test",
			jobShouldBeCreated:                     true,
			clusterObjectDeploymentShouldBeCreated: false,
			cleanupOnExit:                          true,
			expectedPackageConditionType:           corev1alpha1.PackageUnpacked,
			expectedPackageConditionStatus:         metav1.ConditionFalse,
		},
	}

	ctx := context.TODO()
	pkoNamespace, err := findPackageOperatorNamespace(context.TODO())
	require.NoError(t, err)
	defer func() {
		for _, testCase := range testCases {
			if testCase.cleanupOnExit {
				job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "job-" + testCase.packageName, Namespace: pkoNamespace}}
				err = Client.Delete(ctx, job)
				require.NoError(t, client.IgnoreNotFound(err))
			}
		}
	}()

	for _, testCase := range testCases {
		pkg := clusterPackageGen(testCase.packageName, testCase.packageImage)

		err = Client.Create(ctx, pkg)
		require.NoError(t, err)

		expectedJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "job-" + pkg.Name, Namespace: pkoNamespace}}
		if testCase.jobShouldBeCreated {
			err = Waiter.WaitForObject(ctx, expectedJob, "waiting for the job to be created", func(obj client.Object) (done bool, err error) { return true, nil })
			require.NoError(t, err)
		}

		expectedClusterObjectDeployment := &corev1alpha1.ClusterObjectDeployment{ObjectMeta: metav1.ObjectMeta{Name: pkg.Name}}
		if testCase.clusterObjectDeploymentShouldBeCreated {
			err = Waiter.WaitForObject(ctx, expectedClusterObjectDeployment, "waiting for the clusterObjectDeployment to be created", func(obj client.Object) (done bool, err error) { return true, nil })
			require.NoError(t, err)
		}

		err = Waiter.WaitForCondition(ctx, pkg, testCase.expectedPackageConditionType, testCase.expectedPackageConditionStatus)
		require.NoError(t, err)

		// test deletion
		err = Client.Delete(ctx, pkg)
		require.NoError(t, err)

		err = Waiter.WaitToBeGone(ctx, pkg, func(obj client.Object) (done bool, err error) { return false, nil })
		require.NoError(t, err)

		err = Waiter.WaitToBeGone(ctx, expectedJob, func(obj client.Object) (done bool, err error) { return false, nil })
		require.NoError(t, err)

		err = Waiter.WaitToBeGone(ctx, expectedClusterObjectDeployment, func(obj client.Object) (done bool, err error) { return false, nil })
		require.NoError(t, err)
	}
}
