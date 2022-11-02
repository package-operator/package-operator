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

func TestPackage_success(t *testing.T) {
	tests := []struct {
		name             string
		pkg              client.Object
		objectDeployment client.Object
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
				},
			},
			objectDeployment: &corev1alpha1.ClusterObjectDeployment{},
		},
		{
			name: "namespaced with slices",
			pkg: &corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "success",
					Namespace: "default",
					Annotations: map[string]string{
						"package-operator.run/test-stub-image":            TestStubImage,
						"packages.package-operator.run/chunking-strategy": "EachObject",
					},
				},
				Spec: corev1alpha1.PackageSpec{
					Image: SuccessTestPackageImage,
				},
			},
			objectDeployment: &corev1alpha1.ObjectDeployment{},
		},
		{
			name: "cluster with slices",
			pkg: &corev1alpha1.ClusterPackage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "success",
					Annotations: map[string]string{
						"package-operator.run/test-stub-image":            TestStubImage,
						"packages.package-operator.run/chunking-strategy": "EachObject",
					},
				},
				Spec: corev1alpha1.PackageSpec{
					Image: SuccessTestPackageImage,
				},
			},
			objectDeployment: &corev1alpha1.ClusterObjectDeployment{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := logr.NewContext(context.Background(), testr.New(t))

			require.NoError(t, Client.Create(ctx, test.pkg))
			cleanupOnSuccess(ctx, t, test.pkg)

			require.NoError(t,
				Waiter.WaitForCondition(ctx, test.pkg, corev1alpha1.PackageInvalid, metav1.ConditionFalse))
			require.NoError(t,
				Waiter.WaitForCondition(ctx, test.pkg, corev1alpha1.PackageUnpacked, metav1.ConditionTrue))
			require.NoError(t,
				Waiter.WaitForCondition(ctx, test.pkg, corev1alpha1.PackageAvailable, metav1.ConditionTrue))

			require.NoError(t, Client.Get(ctx, client.ObjectKey{
				Name: test.pkg.GetName(), Namespace: test.pkg.GetNamespace(),
			}, test.objectDeployment))
		})
	}
}
