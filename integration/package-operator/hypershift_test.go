//go:build integration

package packageoperator

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"pkg.package-operator.run/cardboard/kubeutils/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	hypershiftv1beta1 "package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
)

func TestHyperShift(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	t.Run("HyperShift Installation", func(t *testing.T) {
		installHyperShift(ctx, t)
	})

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

	// copy service-network-admin-kubeconfig from default namespace
	defaultSecret := &corev1.Secret{}
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name:      "service-network-admin-kubeconfig",
		Namespace: "default",
	}, defaultSecret))
	hcSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service-network-admin-kubeconfig",
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

	// Wait for roll-out of remote phase package
	rpPkg := &corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "remote-phase",
			Namespace: ns.Name,
		},
	}
	// longer timeout because PKO is restarting to enable HyperShift integration and needs a
	// few seconds for leader election.
	err = Waiter.WaitForCondition(
		ctx, rpPkg, corev1alpha1.PackageAvailable,
		metav1.ConditionTrue, wait.WithTimeout(100*time.Second),
	)
	require.NoError(t, err)

	// Wait for roll-out of hosted cluster package
	hcPkg := &corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hosted-cluster",
			Namespace: ns.Name,
		},
	}
	err = Waiter.WaitForCondition(
		ctx, hcPkg, corev1alpha1.PackageAvailable,
		metav1.ConditionTrue, wait.WithTimeout(100*time.Second),
	)
	require.NoError(t, err)

	// Test ObjectSetPhase integration
	t.Run("ObjectSetSetupPauseTeardown", func(t *testing.T) {
		runObjectSetSetupPauseTeardownTest(t, ns.Name, "hosted-cluster")
	})
	t.Run("ObjectSetHandover", func(t *testing.T) {
		runObjectSetHandoverTest(t, ns.Name, "hosted-cluster")
	})
	t.Run("ObjectSetOrphanCascadeDeletion", func(t *testing.T) {
		t.SkipNow() // This test/functionality is not stable.
		runObjectSetOrphanCascadeDeletionTest(t, ns.Name, "hosted-cluster")
	})
}

func installHyperShift(ctx context.Context, t *testing.T) {
	t.Helper()

	// TODO: this comment is wrong?!
	// tests that PackageOperator will deploy a new remote-phase-manager
	// for every ready HyperShift HostedCluster.
	hostedClusterCRDBytes, err := os.ReadFile("testdata/hostedclusters.crd.yaml")
	require.NoError(t, err)
	hostedClusterCRD := &unstructured.Unstructured{}
	require.NoError(t, yaml.Unmarshal(hostedClusterCRDBytes, hostedClusterCRD))
	require.NoError(t, Client.Create(ctx, hostedClusterCRD))
	require.NoError(t, Waiter.WaitForCondition(ctx, hostedClusterCRD, "Established", metav1.ConditionTrue))

	require.NoError(t, initClients(ctx))
}
