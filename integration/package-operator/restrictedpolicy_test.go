//go:build integration

package packageoperator

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This test validates that the package-operator namespace is configured to actually enforce the restricted PSS.
func TestRestrictedPolicyPodCreation(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testing-policy-pod",
			Namespace: "package-operator-system",
			Annotations: map[string]string{
				"description": "This is a test pod",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "my-container",
					Image: "nginx:latest",
				},
			},
		},
	}

	ctx := logr.NewContext(t.Context(), testr.New(t))
	err := Client.Create(ctx, pod)
	require.ErrorContains(t, err, `forbidden: violates PodSecurity "restricted:latest": `)
	defer cleanupOnSuccess(ctx, t, pod)
}
