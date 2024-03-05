package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
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
		run.Meth(cluster, cluster.create),
		run.Meth(generate, generate.All),
	); err != nil {
		return err
	}

	var f string
	if len(filter) > 0 {
		f = "-run " + filter
	}

	cl, err := cluster.Clients()
	if err != nil {
		return err
	}

	err = cl.CreateAndWaitFromFiles(ctx, []string{filepath.Join("config", "self-bootstrap-job-local.yaml")})
	if err != nil {
		return err
	}

	// Bootstrap job is cleaning itself up after completion, so we can't wait for Condition Completed=True.
	// See self-bootstrap-job .spec.ttlSecondsAfterFinished: 0
	err = cl.Waiter.WaitToBeGone(ctx,
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: "package-operator-bootstrap", Namespace: "package-operator-system"},
		},
		func(obj client.Object) (done bool, err error) { return },
	)
	if err != nil {
		return err
	}

	err = cl.Waiter.WaitForCondition(ctx,
		&corev1alpha1.ClusterPackage{
			ObjectMeta: metav1.ObjectMeta{
				Name: "package-operator",
			},
		},
		corev1alpha1.PackageAvailable,
		metav1.ConditionTrue,
	)
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
	_ = cl.CtrlClient.Delete(ctx, secret)
	if err := cl.CtrlClient.Create(ctx, secret); err != nil {
		return fmt.Errorf("deploy kubeconfig secret: %w", err)
	}

	kubeconfigPath, err := cluster.KubeconfigPath()
	if err != nil {
		return err
	}

	env := sh.WithEnvironment{
		"CGO_ENABLED":                          "1",
		"PKO_TEST_SUCCESS_PACKAGE_IMAGE":       imageURL(imageRegistry(), "test-stub-package", appVersion),
		"PKO_TEST_SUCCESS_MULTI_PACKAGE_IMAGE": imageURL(imageRegistry(), "test-stub-multi-package", appVersion),
		"PKO_TEST_SUCCESS_CEL_PACKAGE_IMAGE":   imageURL(imageRegistry(), "test-stub-cel-package", appVersion),
		"PKO_TEST_STUB_IMAGE":                  imageURL(imageRegistry(), "test-stub", appVersion),
		"PKO_TEST_LATEST_BOOTSTRAP_JOB":        os.Getenv("PKO_TEST_LATEST_BOOTSTRAP_JOB"),
		"KUBECONFIG":                           kubeconfigPath,
	}

	if env["PKO_TEST_LATEST_BOOTSTRAP_JOB"] == "" {
		url := "https://github.com/package-operator/package-operator/releases/latest/download/self-bootstrap-job.yaml"
		env["PKO_TEST_LATEST_BOOTSTRAP_JOB"] = url
	}

	goTestCmd := t.makeGoIntTestCmd(f, jsonOutput)

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
	default:
		return nil
	}
}

func (Test) makeGoIntTestCmd(filter string, jsonOutput bool) string {
	args := []string{
		"go", "test",
		"-tags=integration",
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
		args = append(args, "|", "gotestfmt", "--hide=empty-packages")
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
		fmt.Sprintf(`go test %s ./... 2>&1 | tee "%s" | gotestfmt --hide=empty-packages`, argStr, logPath),
	)
}
