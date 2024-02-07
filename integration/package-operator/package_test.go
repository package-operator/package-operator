//go:build integration

package packageoperator

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func TestPackage_success(t *testing.T) {
	tests := []struct {
		name             string
		pkg              client.Object
		objectDeployment client.Object
		postCheck        func(ctx context.Context, t *testing.T)
	}{
		{
			name: "simple/namespaced",
			pkg: &corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "success",
					Namespace: "default",
				},
				Spec: corev1alpha1.PackageSpec{
					Image: SuccessTestPackageImage,
					Config: &runtime.RawExtension{
						Raw: []byte(fmt.Sprintf(`{"testStubImage": "%s"}`, TestStubImage)),
					},
				},
			},
			objectDeployment: &corev1alpha1.ObjectDeployment{},
			postCheck: func(ctx context.Context, t *testing.T) {
				t.Helper()

				// Test if environment information is injected successfully.
				deploy := &appsv1.Deployment{}
				err := Client.Get(ctx, client.ObjectKey{
					Name:      "test-stub-success",
					Namespace: "default",
				}, deploy)
				require.NoError(t, err)

				var env manifestsv1alpha1.PackageEnvironment
				te := deploy.Annotations["test-environment"]
				err = json.Unmarshal([]byte(te), &env)
				require.NoError(t, err)
				assert.NotEmpty(t, env.Kubernetes.Version)
			},
		},
		{
			name: "simple/cluster",
			pkg: &corev1alpha1.ClusterPackage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "success",
				},
				Spec: corev1alpha1.PackageSpec{
					Image: SuccessTestPackageImage,
					Config: &runtime.RawExtension{
						Raw: []byte(fmt.Sprintf(`{"testStubImage": "%s"}`, TestStubImage)),
					},
				},
			},
			objectDeployment: &corev1alpha1.ClusterObjectDeployment{},
		},
		{
			name: "simple/namespaced with slices",
			pkg: &corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "success-slices",
					Namespace: "default",
					Annotations: map[string]string{
						"packages.package-operator.run/chunking-strategy": "EachObject",
					},
				},
				Spec: corev1alpha1.PackageSpec{
					Image: SuccessTestPackageImage,
					Config: &runtime.RawExtension{
						Raw: []byte(fmt.Sprintf(`{"testStubImage": "%s"}`, TestStubImage)),
					},
				},
			},
			objectDeployment: &corev1alpha1.ObjectDeployment{},
			postCheck: func(ctx context.Context, t *testing.T) {
				t.Helper()
				sliceList := &corev1alpha1.ObjectSliceList{}
				err := Client.List(ctx, sliceList)
				require.NoError(t, err)

				// Just a Deployment
				assertLenWithJSON(t, sliceList.Items, 1)
			},
		},
		{
			name: "simple/cluster with slices",
			pkg: &corev1alpha1.ClusterPackage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "success-slices",
					Annotations: map[string]string{
						"package-operator.run/test-stub-image":            TestStubImage,
						"packages.package-operator.run/chunking-strategy": "EachObject",
					},
				},
				Spec: corev1alpha1.PackageSpec{
					Image: SuccessTestPackageImage,
					Config: &runtime.RawExtension{
						Raw: []byte(fmt.Sprintf(`{"testStubImage": "%s"}`, TestStubImage)),
					},
				},
			},
			objectDeployment: &corev1alpha1.ClusterObjectDeployment{},
			postCheck: func(ctx context.Context, t *testing.T) {
				t.Helper()
				sliceList := &corev1alpha1.ClusterObjectSliceList{}
				err := Client.List(ctx, sliceList)
				require.NoError(t, err)

				// Namespace and Deployment
				assertLenWithJSON(t, sliceList.Items, 2)
			},
		},
		{
			name: "multi/namespaced",
			pkg: &corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "success-multi",
					Namespace: "default",
				},
				Spec: corev1alpha1.PackageSpec{
					Image: SuccessTestMultiPackageImage,
					Config: &runtime.RawExtension{
						Raw: []byte(fmt.Sprintf(`{"testStubMultiPackageImage": "%s","testStubImage": "%s"}`,
							SuccessTestMultiPackageImage, TestStubImage,
						)),
					},
				},
			},
			objectDeployment: &corev1alpha1.ObjectDeployment{},
			postCheck: func(ctx context.Context, t *testing.T) {
				t.Helper()

				// test if the dependent packages exist
				pkgBE := &corev1alpha1.Package{}
				err := Client.Get(ctx, client.ObjectKey{Name: "success-multi-backend", Namespace: "default"}, pkgBE)
				require.NoError(t, err)
				pkgFE := &corev1alpha1.Package{}
				err = Client.Get(ctx, client.ObjectKey{Name: "success-multi-frontend", Namespace: "default"}, pkgFE)
				require.NoError(t, err)
			},
		},
		{
			name: "multi/cluster",
			pkg: &corev1alpha1.ClusterPackage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "success-multi",
				},
				Spec: corev1alpha1.PackageSpec{
					Image: SuccessTestMultiPackageImage,
					Config: &runtime.RawExtension{
						Raw: []byte(fmt.Sprintf(`{"testStubMultiPackageImage": "%s","testStubImage": "%s"}`,
							SuccessTestMultiPackageImage, TestStubImage,
						)),
					},
				},
			},
			objectDeployment: &corev1alpha1.ClusterObjectDeployment{},
			postCheck: func(ctx context.Context, t *testing.T) {
				t.Helper()

				// test if the dependent packages exist
				pkgBE := &corev1alpha1.ClusterPackage{}
				err := Client.Get(ctx, client.ObjectKey{Name: "success-multi-backend", Namespace: "success-multi"}, pkgBE)
				require.NoError(t, err)
				pkgFE := &corev1alpha1.ClusterPackage{}
				err = Client.Get(ctx, client.ObjectKey{Name: "success-multi-frontend", Namespace: "success-multi"}, pkgFE)
				require.NoError(t, err)
			},
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			ctx := logr.NewContext(context.Background(), testr.New(t))

			require.NoError(t, Client.Create(ctx, test.pkg))
			cleanupOnSuccess(ctx, t, test.pkg)

			require.NoError(t,
				Waiter.WaitForCondition(ctx, test.pkg, corev1alpha1.PackageUnpacked, metav1.ConditionTrue))
			// Condition Mapping from Deployment
			require.NoError(t,
				Waiter.WaitForCondition(ctx, test.pkg, "my-prefix/Progressing", metav1.ConditionTrue))
			require.NoError(t,
				Waiter.WaitForCondition(ctx, test.pkg, corev1alpha1.PackageAvailable, metav1.ConditionTrue))

			require.NoError(t, Client.Get(ctx, client.ObjectKey{
				Name: test.pkg.GetName(), Namespace: test.pkg.GetNamespace(),
			}, test.objectDeployment))

			if test.postCheck != nil {
				test.postCheck(ctx, t)
			}
		})
	}
}

func TestPackage_nonExistent(t *testing.T) {
	pkg := &corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-existent",
			Namespace: "default",
		},
		Spec: corev1alpha1.PackageSpec{
			Image: "quay.io/package-operator/non-existent:v123",
		},
	}

	ctx := logr.NewContext(context.Background(), testr.New(t))
	require.NoError(t, Client.Create(ctx, pkg))
	cleanupOnSuccess(ctx, t, pkg)

	require.NoError(t,
		Waiter.WaitForCondition(ctx, pkg, corev1alpha1.PackageUnpacked, metav1.ConditionFalse))

	existingPackage := &corev1alpha1.Package{}
	err := Client.Get(ctx, client.ObjectKey{Name: "non-existent", Namespace: "default"}, existingPackage)
	require.NoError(t, err)
	require.Equal(t, "ImagePullBackOff", existingPackage.Status.Conditions[0].Reason)
}

// assert len but print json output.
func assertLenWithJSON[T any](t *testing.T, obj []T, l int) {
	t.Helper()
	if len(obj) == l {
		return
	}

	j, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	t.Error(fmt.Sprintf("should be of len %d", l), string(j))
}
