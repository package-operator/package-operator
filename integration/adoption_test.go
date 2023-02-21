package integration

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coordinationv1alpha1 "package-operator.run/apis/coordination/v1alpha1"
)

func TestAdoption(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "adoption-cm1",
			Namespace: "default",
		},
	}

	adoption := &coordinationv1alpha1.Adoption{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cms",
			Namespace: "default",
		},
		Spec: coordinationv1alpha1.AdoptionSpec{
			Strategy: coordinationv1alpha1.AdoptionStrategy{
				Type: coordinationv1alpha1.AdoptionStrategyStatic,
				Static: &coordinationv1alpha1.AdoptionStrategyStaticSpec{
					Labels: map[string]string{
						"op-ver": "v2",
					},
				},
			},
			TargetAPI: coordinationv1alpha1.TargetAPI{
				Version: "v1",
				Kind:    "ConfigMap",
			},
		},
	}

	err := Client.Create(ctx, cm1)
	require.NoError(t, err)
	defer cleanupOnSuccess(ctx, t, cm1)

	err = Client.Create(ctx, adoption)
	require.NoError(t, err)
	defer cleanupOnSuccess(ctx, t, adoption)

	err = Waiter.WaitForObject(
		ctx, cm1, "waiting for labels",
		func(obj client.Object) (done bool, err error) {
			cm := obj.(*corev1.ConfigMap)
			return cm.Labels["op-ver"] == "v2", nil
		})
	require.NoError(t, err)
}
