package packageoperator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/mt-sre/devkube/dev"
	"github.com/stretchr/testify/require"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pkocore "package-operator.run/apis/core/v1alpha1"
)

const (
	UpgradeTestWaitTimeout            = 5 * time.Minute
	PackageOperatorClusterPackageName = "package-operator"
)

func TestUpgrade(t *testing.T) {
	log := testr.New(t)
	ctx := logr.NewContext(context.Background(), log)

	// package that is installed to ensure the teardown job cleans up packages.
	payloadPkg := &pkocore.Package{
		ObjectMeta: meta.ObjectMeta{
			Name:      "success",
			Namespace: "default",
		},
		Spec: pkocore.PackageSpec{
			Image: SuccessTestPackageImage,
			Config: &runtime.RawExtension{
				Raw: []byte(fmt.Sprintf(`{"testStubImage": "%s"}`, TestStubImage)),
			},
		},
	}

	pkoPackage := &pkocore.ClusterPackage{ObjectMeta: meta.ObjectMeta{Name: PackageOperatorClusterPackageName}}
	pkoNamespace := &core.Namespace{ObjectMeta: meta.ObjectMeta{Name: "package-operator-system"}}

	selector, err := labels.ValidatedSelectorFromSet(map[string]string{"package-operator.run/package": "package-operator"})
	if err != nil {
		panic(err)
	}

	// Install payload package.
	require.NoError(t, Client.Create(ctx, payloadPkg))
	require.NoError(t, Waiter.WaitForCondition(ctx, payloadPkg, pkocore.PackageAvailable, meta.ConditionTrue))

	// Delete existing PKO installation and wait for it to be gone.
	require.NoError(t, Client.Delete(ctx, pkoPackage))

	// Ensure PKO namespace and package is gone
	require.NoError(t, Waiter.WaitToBeGone(ctx, pkoNamespace, func(obj client.Object) (done bool, err error) { return }, dev.WithTimeout(30*time.Minute)))
	require.NoError(t, Waiter.WaitToBeGone(ctx, pkoPackage, func(obj client.Object) (done bool, err error) { return }, dev.WithTimeout(30*time.Minute)))

	// Ensure PKO CRDs are gone.
	crdl := ext.CustomResourceDefinitionList{}
	require.NoError(t, Client.List(ctx, &crdl, &client.ListOptions{LabelSelector: selector}))

	for i := range crdl.Items {
		require.NoError(t, Waiter.WaitToBeGone(
			ctx,
			&crdl.Items[i],
			func(obj client.Object) (done bool, err error) { return },
			dev.WithTimeout(15*time.Minute),
		))
	}

	log.Info("Installing latest released PKO", "job", LatestSelfBootstrapJobURL)
	require.NoError(t, createAndWaitFromHTTP(ctx, []string{LatestSelfBootstrapJobURL}))
	assertInstallDone(ctx, t, pkoPackage)
	log.Info("Latest released PKO is now available")

	log.Info("Apply self-bootstrap-job.yaml built from sources")
	require.NoError(t, createAndWaitFromFiles(ctx, []string{filepath.Join("..", "..", "config", "self-bootstrap-job.yaml")}))
	assertInstallDone(ctx, t, pkoPackage)
}

func assertInstallDone(ctx context.Context, t *testing.T, pkg *pkocore.ClusterPackage) {
	t.Helper()
	jobList := &batch.JobList{}
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
			pkocore.PackageProgressing, meta.ConditionFalse,
			dev.WithTimeout(UpgradeTestWaitTimeout)))
	require.NoError(t,
		Waiter.WaitForCondition(ctx, pkg,
			pkocore.PackageAvailable, meta.ConditionTrue,
			dev.WithTimeout(UpgradeTestWaitTimeout)))
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
func createAndWaitForReadiness(ctx context.Context, object client.Object) error {
	if err := Client.Create(ctx, object); err != nil &&
		!k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating object: %w", err)
	}

	if err := Waiter.WaitForReadiness(ctx, object); err != nil {
		var unknownTypeErr *dev.UnknownTypeError
		if errors.As(err, &unknownTypeErr) {
			// A lot of types don't require waiting for readiness,
			// so we should not error in cases when object types
			// are not registered for the generic wait method.
			return nil
		}

		return fmt.Errorf("waiting for object: %w", err)
	}
	return nil
}
