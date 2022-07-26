package integration

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	hypershiftv1beta1 "package-operator.run/package-operator/internal/controllers/hostedclusters/hypershift/v1beta1"
)

func TestHyperShift(t *testing.T) {
	// tests that PackageOperator will deploy a new remote-phase-manager
	// for every ready HyperShift HostedCluster.
	ctx := logr.NewContext(context.Background(), testr.New(t))

	hc := &hypershiftv1beta1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hc",
			Namespace: "default",
		},
	}
	err := Client.Create(ctx, hc)
	require.NoError(t, err)
	defer cleanupOnSuccess(ctx, t, hc)

	// Simulate HS cluster namespace setup.
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-test-hc",
		},
	}
	err = Client.Create(ctx, ns)
	require.NoError(t, err)
	defer cleanupOnSuccess(ctx, t, ns)

	// copy admin-kubeconfig from default namespace
	defaultSecret := &corev1.Secret{}
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name:      "admin-kubeconfig",
		Namespace: "default",
	}, defaultSecret))
	hcSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "admin-kubeconfig",
			Namespace: ns.Name,
		},
		Data: defaultSecret.Data,
	}
	require.NoError(t, Client.Create(ctx, hcSecret))

	meta.SetStatusCondition(&hc.Status.Conditions, metav1.Condition{
		Type:   hypershiftv1beta1.HostedClusterAvailable,
		Reason: "Success",
		Status: metav1.ConditionTrue,
	})
	err = Client.Status().Update(ctx, hc)
	require.NoError(t, err)

	// Wait for roll out
	pkg := &corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "remote-phase",
			Namespace: ns.Name,
		},
	}
	require.NoError(t,
		Waiter.WaitForCondition(ctx, pkg, corev1alpha1.PackageAvailable, metav1.ConditionTrue))

	// Test ObjectSetPhase integration
	t.Run("ObjectSetSetupPauseTeardown", func(t *testing.T) {
		runObjectSetSetupPauseTeardownTest(t, ns.Name, "hosted-cluster")
	})
	t.Run("ObjectSetHandover", func(t *testing.T) {
		runObjectSetHandoverTest(t, ns.Name, "hosted-cluster")
	})
}
