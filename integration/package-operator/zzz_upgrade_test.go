package packageoperator

import (
	"bytes"
	"context"
	goerrors "errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/mt-sre/devkube/dev"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const UpgradeTestWaitTimeout = 5 * time.Minute

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

	log.Info("Installing latest released PKO", "job", LatestSelfBootstrapJobURL)
	require.NoError(t, createAndWaitFromHTTP(ctx, []string{LatestSelfBootstrapJobURL}))

	jobList := &batchv1.JobList{}
	require.NoError(t, Client.List(
		ctx, jobList,
		client.InNamespace(PackageOperatorNamespace),
	))
	for i := range jobList.Items {
		require.NoError(t,
			Waiter.WaitToBeGone(ctx, &jobList.Items[i],
				func(obj client.Object) (done bool, err error) { return false, nil },
				dev.WithTimeout(UpgradeTestWaitTimeout)))
	}

	require.NoError(t,
		Waiter.WaitForCondition(ctx, pkg,
			corev1alpha1.PackageAvailable, metav1.ConditionTrue,
			dev.WithTimeout(UpgradeTestWaitTimeout)))
	log.Info("Latest released PKO is now available")

	log.Info("Apply self-bootstrap-job.yaml built from sources")
	require.NoError(t, createAndWaitFromFiles(ctx, []string{filepath.Join("..", "..", "config", "self-bootstrap-job.yaml")}))

	jobList = &batchv1.JobList{}
	require.NoError(t, Client.List(
		ctx, jobList,
		client.InNamespace(PackageOperatorNamespace),
	))
	for i := range jobList.Items {
		require.NoError(t,
			Waiter.WaitToBeGone(ctx, &jobList.Items[i],
				func(obj client.Object) (done bool, err error) { return false, nil },
				dev.WithTimeout(UpgradeTestWaitTimeout)))
	}

	require.NoError(t,
		Waiter.WaitForCondition(ctx, pkg,
			corev1alpha1.PackageProgressing, metav1.ConditionFalse,
			dev.WithTimeout(UpgradeTestWaitTimeout)))
	require.NoError(t,
		Waiter.WaitForCondition(ctx, pkg,
			corev1alpha1.PackageAvailable, metav1.ConditionTrue,
			dev.WithTimeout(UpgradeTestWaitTimeout)))
}

func deleteExistingPKO(ctx context.Context) error {
	log := logr.FromContextOrDiscard(ctx)

	cosList := &corev1alpha1.ClusterObjectSetList{}
	err := Client.List(
		ctx, cosList,
		client.MatchingLabels{
			"package-operator.run/object-deployment": "package-operator",
		},
	)
	if err != nil {
		return err
	}
	for i := range cosList.Items {
		cos := &cosList.Items[i]
		log.Info(fmt.Sprintf("Removing finalizers from cluster object set %s: %v", cos.Name, cos.Finalizers))
		patch := fmt.Sprintf("{\"metadata\":{\"resourceVersion\":\"%s\",\"finalizers\":[]}}", cos.ResourceVersion)
		err = Client.Patch(ctx, cos, client.RawPatch(types.MergePatchType, []byte(patch)))
		if err != nil {
			return err
		}
	}

	pkg := &corev1alpha1.ClusterPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "package-operator",
			Namespace: PackageOperatorNamespace,
		},
	}

	log.Info("Deleting existing PKO")
	err = Client.Delete(ctx, pkg)
	if err != nil {
		return err
	}

	err = Waiter.WaitToBeGone(ctx, pkg, func(obj client.Object) (done bool, err error) { return false, nil })
	if err != nil {
		return err
	}

	err = Waiter.WaitToBeGone(ctx,
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: PackageOperatorNamespace}},
		func(obj client.Object) (done bool, err error) { return false, nil },
		dev.WithTimeout(UpgradeTestWaitTimeout),
	)
	if err != nil {
		return err
	}

	log.Info("Existing PKO fully deleted")
	return nil
}

func createAndWaitFromFiles(ctx context.Context, files []string) error {
	var objects []unstructured.Unstructured
	for _, file := range files {
		objs, err := dev.LoadKubernetesObjectsFromFile(file)
		if err != nil {
			return fmt.Errorf("loading objects from file %q: %w", file, err)
		}

		objects = append(objects, objs...)
	}

	for i := range objects {
		if err := createAndWaitForReadiness(ctx, &objects[i]); err != nil {
			return fmt.Errorf("creating object: %w", err)
		}
	}
	return nil
}

func createAndWaitFromHTTP(ctx context.Context, urls []string) error {
	var client http.Client
	var objects []unstructured.Unstructured
	for _, url := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("getting %q: %w", url, err)
		}
		defer resp.Body.Close()

		var content bytes.Buffer
		if _, err := io.Copy(&content, resp.Body); err != nil {
			return fmt.Errorf("reading response %q: %w", url, err)
		}

		objs, err := dev.LoadKubernetesObjectsFromBytes(content.Bytes())
		if err != nil {
			return fmt.Errorf("loading objects from %q: %w", url, err)
		}

		objects = append(objects, objs...)
	}

	for i := range objects {
		if err := createAndWaitForReadiness(ctx, &objects[i]); err != nil {
			return fmt.Errorf("creating object: %w", err)
		}
	}
	return nil
}

// Creates the given objects and waits for them to be considered ready.
func createAndWaitForReadiness(
	ctx context.Context, object client.Object,
) error {
	if err := Client.Create(ctx, object); err != nil &&
		!errors.IsAlreadyExists(err) {
		return fmt.Errorf("creating object: %w", err)
	}

	if err := Waiter.WaitForReadiness(ctx, object); err != nil {
		var unknownTypeErr *dev.UnknownTypeError
		if goerrors.As(err, &unknownTypeErr) {
			// A lot of types don't require waiting for readiness,
			// so we should not error in cases when object types
			// are not registered for the generic wait method.
			return nil
		}

		return fmt.Errorf("waiting for object: %w", err)
	}
	return nil
}
