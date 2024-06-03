//go:build integration

package packageoperator

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRestrictedPolicyPod_creation_(t *testing.T) {

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
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
						},
					},
				},
			},
		},
	}

	ctx := logr.NewContext(context.Background(), testr.New(t))

	err := Client.Create(ctx, pod)
	t.Log(err)
	require.Error(t, err)
	defer cleanupOnSuccess(ctx, t, pod)

}
