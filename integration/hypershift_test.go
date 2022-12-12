package integration

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	hypershiftv1alpha1 "package-operator.run/package-operator/internal/controllers/hostedclusters/hypershift/v1alpha1"
)

func TestHyperShift(t *testing.T) {
	// tests that PackageOperator will deploy a new remote-phase-manager
	// for every ready HyperShift HostedCluster.
	ctx := logr.NewContext(context.Background(), testr.New(t))

	hc := &hypershiftv1alpha1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hc",
			Namespace: "default",
		},
	}
	err := Client.Create(ctx, hc)
	require.NoError(t, err)
	defer cleanupOnSuccess(ctx, t, hc)

	meta.SetStatusCondition(&hc.Status.Conditions, metav1.Condition{
		Type:   hypershiftv1alpha1.HostedClusterAvailable,
		Reason: "Success",
		Status: metav1.ConditionTrue,
	})
	err = Client.Status().Update(ctx, hc)
	require.NoError(t, err)

	// Wait for roll out
	pkg := &corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hc-remote-phase",
			Namespace: "default",
		},
	}
	require.NoError(t,
		Waiter.WaitForCondition(ctx, pkg, corev1alpha1.PackageAvailable, metav1.ConditionTrue))

	// Test ObjectSetPhase integration
	t.Run("ObjectSetSetupPauseTeardown", func(t *testing.T) {
		runObjectSetSetupPauseTeardownTest(t, "hosted-cluster")
	})
	t.Run("ObjectSetHandover", func(t *testing.T) {
		runObjectSetHandoverTest(t, "hosted-cluster")
	})
}
