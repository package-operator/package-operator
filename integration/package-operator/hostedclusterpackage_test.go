//go:build integration_hypershift

package packageoperator

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
)

func TestHostedClusterPackage_InstantRollout(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	hcpkg := &corev1alpha1.HostedClusterPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-hostedcluster-package",
		},
		Spec: corev1alpha1.HostedClusterPackageSpec{
			HostedClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"hcpkg-enable": "True",
				},
			},
			Template: corev1alpha1.PackageTemplateSpec{
				Spec: corev1alpha1.PackageSpec{
					Image: SuccessTestPackageImage,
					Config: &runtime.RawExtension{
						Raw: []byte(fmt.Sprintf(`{"testStubImage": "%s"}`, TestStubImage)),
					},
				},
			},
		},
	}

	require.NoError(t, Client.Create(ctx, hcpkg))
	cleanupOnSuccess(ctx, t, hcpkg)

	hc := &v1beta1.HostedCluster{}
	requireClientGet(ctx, t, "pko-hs-hc", "default", hc)

	pkg := &corev1alpha1.Package{}
	requireClientGet(ctx, t, hcpkg.Name, v1beta1.HostedClusterNamespace(*hc), pkg)
	requireCondition(ctx, t, pkg, corev1alpha1.PackageAvailable, metav1.ConditionTrue)

	requireCondition(ctx, t, hcpkg, corev1alpha1.HostedClusterPackageAvailable, metav1.ConditionTrue)
	requireCondition(ctx, t, hcpkg, corev1alpha1.HostedClusterPackageProgressing, metav1.ConditionFalse)
}
