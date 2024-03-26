//go:build integration_hypershift

package packageoperator

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	k8sscheme "k8s.io/client-go/kubernetes/scheme"

	"k8s.io/apimachinery/pkg/runtime"
	"package-operator.run/apis"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"pkg.package-operator.run/cardboard/kubeutils/wait"
)

func TestHyperShift(t *testing.T) {
	namespace := "default-pko-hs-hc"
	ctx := logr.NewContext(context.Background(), testr.New(t))

	require.NoError(t, initClients(ctx))

	// Wait for roll-out of remote phase package
	rpPkg := &corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "remote-phase",
			Namespace: namespace,
		},
	}
	// longer timeout because PKO is restarting to enable HyperShift integration and needs a
	// few seconds for leader election.
	err := Waiter.WaitForCondition(
		ctx, rpPkg, corev1alpha1.PackageAvailable,
		metav1.ConditionTrue, wait.WithTimeout(10000*time.Second),
	)
	require.NoError(t, err)

	// Wait for roll-out of hosted cluster package
	hcPkg := &corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hosted-cluster",
			Namespace: namespace,
		},
	}
	err = Waiter.WaitForCondition(
		ctx, hcPkg, corev1alpha1.PackageAvailable,
		metav1.ConditionTrue, wait.WithTimeout(100*time.Second),
	)
	require.NoError(t, err)

	hClient, hWaiter, err := hostedClusterHandlers()
	require.NoError(t, err)

	// Test ObjectSetPhase integration
	t.Run("ObjectSetSetupPauseTeardown", func(t *testing.T) {
		runObjectSetSetupPauseTeardownTestWithCustomHandlers(t, hClient, hWaiter, namespace, "hosted-cluster")
	})
	t.Run("ObjectSetHandover", func(t *testing.T) {
		runObjectSetHandoverTestWithCustomHandlers(t, hClient, hWaiter, namespace, "hosted-cluster")
	})
	t.Run("ObjectSetOrphanCascadeDeletion", func(t *testing.T) {
		t.SkipNow() // This test/functionality is not stable.
		runObjectSetOrphanCascadeDeletionTestWithCustomHandlers(t, hClient, hWaiter, namespace, "hosted-cluster")
	})
}

func hostedClusterHandlers() (client.Client, *wait.Waiter, error) {
	scheme := runtime.NewScheme()
	schemeBuilder := runtime.SchemeBuilder{
		k8sscheme.AddToScheme,
		apis.AddToScheme,
	}

	if err := schemeBuilder.AddToScheme(scheme); err != nil {
		return nil, nil, fmt.Errorf("adding defaults to scheme: %w", err)
	}

	kubeconfigPath := filepath.Join("..", "..", ".cache", "clusters", "pko-hs-hc", "kubeconfig.yaml")
	// Create RestConfig
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("getting rest.Config from kubeconfig: %w", err)
	}

	// Create Controller Runtime Client
	ctrlClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return nil, nil, fmt.Errorf("creating new ctrl client: %w", err)
	}

	waiter := wait.NewWaiter(ctrlClient, scheme)
	return ctrlClient, waiter, nil
}
