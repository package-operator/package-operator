package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"pkg.package-operator.run/cardboard/run"
	"pkg.package-operator.run/cardboard/sh"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Test is a collection of test related functions.
type Test struct{}

// Integration runs local integration tests in a KinD cluster.
func (t Test) Integration(ctx context.Context, jsonOutput bool, filter string) error {
	self := run.Meth2(t, t.Integration, jsonOutput, filter)
	if err := mgr.ParallelDeps(ctx, self,
		run.Fn1(bootstrap, ctx),
	); err != nil {
		return err
	}

	var f string
	if len(filter) > 0 {
		f = fmt.Sprintf(`-run "%s"`, filter)
	}

	cl, err := cluster.Clients()
	if err != nil {
		return err
	}

	internalKubeconfig, err := cluster.Kubeconfig(true)
	if err != nil {
		return err
	}

	// Create a new secret for the kubeconfig.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service-network-admin-kubeconfig",
			Namespace: "default",
		},
		Data: map[string][]byte{"kubeconfig": []byte(internalKubeconfig)},
	}

	// Deploy the secret with the new kubeconfig.
	if err := client.IgnoreNotFound(cl.CtrlClient.Delete(ctx, secret)); err != nil {
		return fmt.Errorf("deleting kubeconfig secret: %w", err)
	}
	if err := cl.CtrlClient.Create(ctx, secret); err != nil {
		return fmt.Errorf("deploy kubeconfig secret: %w", err)
	}

	kubeconfigPath, err := cluster.KubeconfigPath()
	if err != nil {
		return err
	}

	env := sh.WithEnvironment{
		"CGO_ENABLED":                                   "1",
		"PKO_TEST_VERSION":                              appVersion,
		"PKO_TEST_SUCCESS_PACKAGE_IMAGE":                imageURL(imageRegistry(), "test-stub-package", appVersion),
		"PKO_TEST_SUCCESS_PACKAGE_IMAGE_AUTH":           imageURL("dev-registry.dev-registry.svc.cluster.local:5002/package-operator", "test-stub-package", appVersion), //nolint:lll
		"PKO_TEST_SUCCESS_MULTI_PACKAGE_IMAGE":          imageURL(imageRegistry(), "test-stub-multi-package", appVersion),
		"PKO_TEST_SUCCESS_CEL_PACKAGE_IMAGE":            imageURL(imageRegistry(), "test-stub-cel-package", appVersion),
		"PKO_TEST_SUCCESS_PAUSE_PACKAGE_IMAGE":          imageURL(imageRegistry(), "test-stub-pause-package", appVersion),
		"PKO_TEST_SUCCESS_IMAGE_PREFIX_OVERRIDE":        imageURL(imageRegistry(), "test-stub-image-prefix-override-package", appVersion),           //nolint:lll
		"PKO_TEST_SUCCESS_IMAGE_PREFIX_OVERRIDE_MIRROR": imageURL(imageRegistry()+"/mirror", "test-stub-image-prefix-override-package", appVersion), //nolint:lll
		"PKO_TEST_STUB_IMAGE":                           imageURL(imageRegistry(), "test-stub", appVersion),
		"PKO_TEST_STUB_IMAGE_SRC":                       imageURL(imageRegistry()+"/src", "test-stub-mirror", appVersion),
		"PKO_TEST_STUB_IMAGE_MIRROR":                    imageURL(imageRegistry()+"/mirror", "test-stub-mirror", appVersion),
		"PKO_TEST_LATEST_BOOTSTRAP_JOB":                 os.Getenv("PKO_TEST_LATEST_BOOTSTRAP_JOB"),
		"PKO_IMAGE_REGISTRY":                            imageRegistry(),
		"KUBECONFIG":                                    kubeconfigPath,
	}

	if env["PKO_TEST_LATEST_BOOTSTRAP_JOB"] == "" {
		url := "https://github.com/package-operator/package-operator/releases/latest/download/self-bootstrap-job.yaml"
		env["PKO_TEST_LATEST_BOOTSTRAP_JOB"] = url
	}

	// standard integration tests
	goTestCmd := t.makeGoIntTestCmd("integration", f, jsonOutput)

	if err := os.MkdirAll(filepath.Join(cacheDir, "integration"), 0o755); err != nil {
		return err
	}

	err = shr.New(env).Bash(goTestCmd)
	eErr := cluster.ExportLogs(filepath.Join(cacheDir, "integration", "logs"))

	switch {
	case err != nil:
		return err
	case eErr != nil:
		return eErr
	}

	// hypershift integration tests
	if err := mgr.SerialDeps(ctx, self,
		run.Meth2(hypershiftHostedCluster, hypershiftHostedCluster.createHostedCluster, &cluster, []string{}),
	); err != nil {
		return err
	}

	goTestHypershiftCmd := t.makeGoIntTestCmd("integration_hypershift", f, jsonOutput)

	if err := os.MkdirAll(filepath.Join(cacheDir, "integration"), 0o755); err != nil {
		return err
	}

	err = shr.New(env).Bash(goTestHypershiftCmd)
	eErr = cluster.ExportLogs(filepath.Join(cacheDir, "integration_hypershift", "logs"))
	eErr2 := hypershiftHostedCluster.ExportLogs(filepath.Join(cacheDir, "integration_hypershift", "logs"))

	switch {
	case err != nil:
		return err
	case eErr != nil:
		return eErr
	case eErr2 != nil:
		return eErr2
	default:
		return nil
	}
}

func (Test) makeGoIntTestCmd(tags string, filter string, jsonOutput bool) string {
	args := []string{
		"go", "test",
		"-tags=" + tags,
		"-coverprofile=" + filepath.Join(cacheDir, "integration", "cover.txt"),
		filter,
		"-race",
		"-test.v",
		"-failfast",
		"-timeout=20m",
		"-count=1",
	}

	if jsonOutput {
		args = append(args, "-json")
	}

	args = append(args,
		"-coverpkg=./...,./apis/...,./pkg/...",
		"./integration/...",
	)

	if jsonOutput {
		args = append(args, "|", "gotestfmt", "--hide=all")
	}

	return strings.Join(args, " ")
}

// Unit runs unittests, the filter argument is passed via -run="".
func (t Test) Unit(_ context.Context, filter string) error {
	if err := os.MkdirAll(filepath.Join(cacheDir, "unit"), 0o755); err != nil {
		return err
	}

	gotestArgs := []string{"-coverprofile=" + filepath.Join(cacheDir, "unit", "cover.txt"), "-race", "-json"}
	if len(filter) > 0 {
		gotestArgs = append(gotestArgs, "-run="+filter)
	}

	argStr := strings.Join(gotestArgs, " ")
	logPath := filepath.Join(cacheDir, "unit", "gotest.log")

	return sh.New(
		sh.WithEnvironment{"CGO_ENABLED": "1"},
	).Bash(
		"set -euo pipefail",
		fmt.Sprintf(`go test %s ./... 2>&1 | tee "%s" | gotestfmt --hide=all`, argStr, logPath),
	)
}
