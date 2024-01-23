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

// internal struct to namespace all test related functions.
type Test struct{}

func (t Test) Integration(ctx context.Context, self run.DependencyIDer, filter string) error {
	var f string
	if len(filter) > 0 {
		f = "-run " + filter
	}

	if err := mgr.SerialDeps(ctx, self, cluster); err != nil {
		return err
	}

	if err := os.MkdirAll(".cache/integration", 0o755); err != nil {
		return err
	}

	cl, err := cluster.Clients()
	if err != nil {
		return err
	}

	err = mgr.ParallelDeps(ctx, self,
		run.Fn2(pushImage, "package-operator-manager", "localhost:5001"),
		run.Fn2(pushImage, "package-operator-webhook", "localhost:5001"),
		run.Fn2(pushImage, "remote-phase-manager", "localhost:5001"),
		run.Fn2(pushImage, "test-stub", "localhost:5001"),
		run.Fn2(pushPackage, "package-operator", "localhost:5001"),
		run.Fn2(pushPackage, "remote-phase", "localhost:5001"),
		run.Fn2(pushPackage, "test-stub", "localhost:5001"),
		run.Fn2(pushPackage, "test-stub-multi", "localhost:5001"),
	)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}

	kubeconfigPath, err := cluster.KubeconfigPath()
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

	// Create a new secret for the kubeconfig
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service-network-admin-kubeconfig",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"kubeconfig": []byte(internalKubeconfig),
		},
	}

	// Deploy the secret with the new kubeconfig
	_ = cl.CtrlClient.Delete(ctx, secret)
	if err := cl.CtrlClient.Create(ctx, secret); err != nil {
		return fmt.Errorf("deploy kubeconfig secret: %w", err)
	}

	packageRegistry := "dev-registry.dev-registry.svc.cluster.local:5001"
	env := sh.WithEnvironment{
		"CGO_ENABLED":                          "1",
		"PKO_TEST_SUCCESS_PACKAGE_IMAGE":       imageURL(packageRegistry, "test-stub-package", appVersion),
		"PKO_TEST_SUCCESS_MULTI_PACKAGE_IMAGE": imageURL(packageRegistry, "test-stub-multi-package", appVersion),
		"PKO_TEST_STUB_IMAGE":                  imageURL("localhost:5001", "test-stub", appVersion),
		"PKO_TEST_LATEST_BOOTSTRAP_JOB":        os.Getenv("PKO_TEST_LATEST_BOOTSTRAP_JOB"),
		"KUBECONFIG":                           kubeconfigPath,
	}

	if env["PKO_TEST_LATEST_BOOTSTRAP_JOB"] == "" {
		d := "https://github.com/package-operator/package-operator/releases/latest/download/self-bootstrap-job.yaml"
		env["PKO_TEST_LATEST_BOOTSTRAP_JOB"] = d
	}

	tArgs := []string{
		"go", "test",
		"-tags=integration", "-coverprofile=.cache/integration/pko-cov.out",
		f, "-race", "-test.v", "-failfast", "-timeout=20m", "-count=1", "-json",
		"-coverpkg=./...,./apis/...,./pkg/...", "./integration/...", "|", "gotestfmt",
	}

	err = shr.New(env).Bash(strings.Join(tArgs, " "))
	eErr := cluster.ExportLogs(".cache/dev-env-logs")

	switch {
	case err != nil:
		return err
	case eErr != nil:
		return eErr
	default:
		return nil
	}
}

// Run unittests, the filter argument is passed via -run="".
func (t Test) Unit(_ context.Context, filter string) error {
	gotestArgs := []string{"-coverprofile=cover.txt", "-race", "-json"}
	if len(filter) > 0 {
		gotestArgs = append(gotestArgs, "-run="+filter)
	}

	argStr := strings.Join(gotestArgs, " ")

	return sh.New(
		sh.WithEnvironment{"CGO_ENABLED": "1"},
	).Bash(
		"set -euo pipefail",
		fmt.Sprintf("go test %s ./... 2>&1 | tee gotest.log | gotestfmt --hide=empty-packages", argStr),
	)
}
