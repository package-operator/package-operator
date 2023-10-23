//go:build integration

package packageoperator

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/mt-sre/devkube/devcluster"
	"github.com/mt-sre/devkube/devos"
	"github.com/stretchr/testify/require"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"

	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	UpgradeTestWaitTimeout            = 5 * time.Minute
	PackageOperatorClusterPackageName = "package-operator"
)

func TestUpgrade(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	require.NoError(t, deleteExistingPKO(ctx))

	log := logr.FromContextOrDiscard(ctx)
	pkg := &corev1alpha1.ClusterPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "package-operator",
			Namespace: PackageOperatorNamespace,
		},
	}
	cluster := devcluster.Cluster{Cli: Client}

	log.Info("Installing latest released PKO", "job", LatestSelfBootstrapJobURL)
	objs, err := devos.UnstructuredFromHTTP(ctx, http.DefaultClient, LatestSelfBootstrapJobURL)
	require.NoError(t, err)
	err = cluster.CreateAndAwaitReadiness(ctx, devos.ObjectsFromUnstructured(objs)...)
	require.NoError(t, err)
	assertInstallDone(ctx, t, pkg)
	log.Info("Latest released PKO is now available")

	log.Info("Apply self-bootstrap-job.yaml built from sources")
	objs, err = devos.UnstructuredFromFiles(nil, filepath.Join("..", "..", "config", "self-bootstrap-job.yaml"))
	require.NoError(t, err)
	err = cluster.CreateAndAwaitReadiness(ctx, devos.ObjectsFromUnstructured(objs)...)
	require.NoError(t, err)
	assertInstallDone(ctx, t, pkg)
}

func assertInstallDone(ctx context.Context, t *testing.T, pkg *corev1alpha1.ClusterPackage) {
	t.Helper()

	poller := Poller
	poller.MaxWaitDuration = UpgradeTestWaitTimeout

	jobList := &batchv1.JobList{}
	require.NoError(t, Client.List(ctx, jobList, client.InNamespace(PackageOperatorNamespace)))
	for i := range jobList.Items {
		require.NoError(t, poller.Wait(ctx, Checker.CheckObj(Client, &jobList.Items[i])))
	}

	require.NoError(t, poller.Wait(ctx, Checker.CheckObj(Client, pkg, CheckClusterPackageNotProgressing)))
	require.NoError(t, poller.Wait(ctx, Checker.CheckObj(Client, pkg, CheckClusterPackageAvailable)))
}

func deleteExistingPKO(ctx context.Context) error {
	log := logr.FromContextOrDiscard(ctx)
	log.Info("Deleting existing PKO")
	packageOperatorPackage := &corev1alpha1.ClusterPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name: PackageOperatorClusterPackageName,
		},
	}

	if err := nukeObject(ctx, packageOperatorPackage); err != nil {
		return fmt.Errorf("stuck PackageOperator ClusterPackage: %w", err)
	}
	log.Info("deleted ClusterPackage", "obj", packageOperatorPackage)

	// Also nuke all the ClusterObjectDeployments belonging to it.
	clusterObjectDeploymentList := &corev1alpha1.ClusterObjectDeploymentList{}
	if err := Client.List(ctx, clusterObjectDeploymentList, client.MatchingLabels{
		manifestsv1alpha1.PackageInstanceLabel: PackageOperatorClusterPackageName,
		manifestsv1alpha1.PackageLabel:         PackageOperatorClusterPackageName,
	}); err != nil {
		return fmt.Errorf("listing stuck PackageOperator ClusterObjectDeployments: %w", err)
	}
	for i := range clusterObjectDeploymentList.Items {
		clusterObjectDeployment := &clusterObjectDeploymentList.Items[i]
		if err := nukeObject(ctx, clusterObjectDeployment); err != nil {
			return fmt.Errorf("stuck PackageOperator ClusterObjectDeployment: %w", err)
		}
		log.Info("deleted ClusterObjectDeployment", "name", clusterObjectDeployment.Name, "obj", clusterObjectDeployment)
	}

	// Also nuke all the ClusterObjectSets belonging to it.
	clusterObjectSetList := &corev1alpha1.ClusterObjectSetList{}
	if err := Client.List(ctx, clusterObjectSetList, client.MatchingLabels{
		manifestsv1alpha1.PackageInstanceLabel: PackageOperatorClusterPackageName,
		manifestsv1alpha1.PackageLabel:         PackageOperatorClusterPackageName,
	}); err != nil {
		return fmt.Errorf("listing stuck PackageOperator ClusterObjectSets: %w", err)
	}
	for i := range clusterObjectSetList.Items {
		clusterObjectSet := &clusterObjectSetList.Items[i]
		if err := nukeObject(ctx, clusterObjectSet); err != nil {
			return fmt.Errorf("stuck PackageOperator ClusterObjectSet: %w", err)
		}
		log.Info("deleted ClusterObjectSet", "name", clusterObjectSet.Name, "obj", clusterObjectSet)
	}

	poller := Poller
	poller.MaxWaitDuration = UpgradeTestWaitTimeout

	if err := poller.Wait(ctx, Checker.CheckGone(Client, packageOperatorPackage)); err != nil {
		return err
	}

	if err := poller.Wait(ctx, Checker.CheckGone(Client, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: PackageOperatorNamespace}})); err != nil {
		return err
	}

	log.Info("Existing PKO fully deleted")
	return nil
}

func nukeObject(ctx context.Context, obj client.Object) error {
	if err := Client.Delete(ctx, obj); apimachineryerrors.IsNotFound(err) {
		// Object already gone.
		return nil
	} else if err != nil {
		return fmt.Errorf("nuking %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}

	return removeAllFinalizersForDeletion(ctx, obj)
}

func removeAllFinalizersForDeletion(ctx context.Context, obj client.Object) error {
	if len(obj.GetFinalizers()) > 0 {
		obj.SetFinalizers([]string{})
		if err := Client.Patch(ctx, obj,
			client.RawPatch(client.Merge.Type(), []byte(`{"metadata": {"finalizers": null}}`))); err != nil && !apimachineryerrors.IsNotFound(err) {
			return fmt.Errorf("releasing finalizers on stuck object %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}
	}
	return nil
}
