package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestPackage_success(t *testing.T) {
	tests := []struct {
		name             string
		pkg              client.Object
		objectDeployment client.Object
		postCheck        func(ctx context.Context, t *testing.T)
	}{
		{
			name: "namespaced",
			pkg: &corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "success",
					Namespace: "default",
					Annotations: map[string]string{
						"package-operator.run/test-stub-image": TestStubImage,
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
		},
		{
			name: "cluster",
			pkg: &corev1alpha1.ClusterPackage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "success",
					Annotations: map[string]string{
						"package-operator.run/test-stub-image": TestStubImage,
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
		},
		{
			name: "namespaced with slices",
			pkg: &corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "success-slices",
					Namespace: "default",
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
			name: "cluster with slices",
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := logr.NewContext(context.Background(), testr.New(t))

			require.NoError(t, Client.Create(ctx, test.pkg))
			cleanupOnSuccess(ctx, t, test.pkg)

			require.NoError(t,
				Waiter.WaitForCondition(ctx, test.pkg, corev1alpha1.PackageUnpacked, metav1.ConditionTrue))
			require.NoError(t,
				Waiter.WaitForCondition(ctx, test.pkg, corev1alpha1.PackageAvailable, metav1.ConditionTrue))

			// Condition Mapping from Deployment
			require.NoError(t,
				Waiter.WaitForCondition(ctx, test.pkg, "my-prefix/Progressing", metav1.ConditionTrue))

			require.NoError(t, Client.Get(ctx, client.ObjectKey{
				Name: test.pkg.GetName(), Namespace: test.pkg.GetNamespace(),
			}, test.objectDeployment))

			if test.postCheck != nil {
				test.postCheck(ctx, t)
			}
		})
	}
}

// assert len but print json output.
func assertLenWithJSON[T interface{}](t *testing.T, obj []T, l int) {
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
