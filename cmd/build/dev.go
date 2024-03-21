package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/unix"
	corev1 "k8s.io/api/core/v1"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"pkg.package-operator.run/cardboard/run"

	hsv1beta1 "package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
)

// Dev focused commands using local development environment.
type Dev struct{}

// PreCommit runs linters and code-gens for pre-commit.
func (dev *Dev) PreCommit(ctx context.Context, args []string) error {
	self := run.Meth1(dev, dev.PreCommit, args)
	return mgr.ParallelDeps(ctx, self,
		run.Meth(generate, generate.All),
		run.Meth(lint, lint.glciFix),
		run.Meth(lint, lint.goModTidyAll),
	)
}

// Generate code, api docs, install files.
func (dev *Dev) Generate(ctx context.Context, args []string) error {
	self := run.Meth1(dev, dev.Generate, args)

	return mgr.ParallelDeps(
		ctx, self,
		run.Meth(generate, generate.code),
		run.Meth(generate, generate.docs),
		run.Meth(generate, generate.installYamlFile),
		run.Meth(generate, generate.selfBootstrapJob),
		run.Meth(generate, generate.selfBootstrapJobLocal),
	)
}

// Unit runs local unittests.
func (dev *Dev) Unit(ctx context.Context, args []string) error {
	var filter string
	switch len(args) {
	case 0:
		// nothing
	case 1:
		filter = args[0]
	default:
		return errors.New("only supports a single argument") //nolint:goerr113
	}
	return test.Unit(ctx, filter)
}

// Integration runs local integration tests in a KinD cluster.
func (dev *Dev) Integration(ctx context.Context, args []string) error {
	var filter string
	switch len(args) {
	case 0:
		// nothing
	case 1:
		filter = args[0]
	default:
		return errors.New("only supports a single argument") //nolint:goerr113
	}
	return test.Integration(ctx, false, filter)
}

// Lint runs local linters to check the codebase.
func (dev *Dev) Lint(_ context.Context, _ []string) error {
	return lint.glciCheck()
}

// LintFix tries to fix linter issues.
func (dev *Dev) LintFix(_ context.Context, _ []string) error {
	return lint.glciFix()
}

// Create the local development cluster.
func (dev *Dev) Create(ctx context.Context, _ []string) error {
	return cluster.create(ctx)
}

// Load CRDs into the local development cluster.
func (dev *Dev) LoadCRDs(ctx context.Context, args []string) error {
	self := run.Meth1(dev, dev.LoadCRDs, args)
	if err := mgr.ParallelDeps(ctx, self,
		run.Meth(generate, generate.code),
		run.Meth(cluster, cluster.create),
	); err != nil {
		return err
	}

	// get cluster clients
	clients, err := cluster.Clients()
	if err != nil {
		return err
	}

	// load CRDs
	entries, err := os.ReadDir(filepath.Join("config", "crds"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			entryPath := filepath.Join("config", "crds", entry.Name())
			if err = clients.CreateAndWaitFromFiles(ctx, []string{entryPath}); err != nil {
				return err
			}
		}
	}

	return nil
}

// Destroy the local development cluster.
func (dev *Dev) Destroy(ctx context.Context, _ []string) error {
	self := run.Meth1(dev, dev.Destroy, []string{})
	return mgr.ParallelDeps(ctx, self,
		run.Meth(cluster, cluster.destroy),
		run.Meth(hypershiftHostedCluster, hypershiftHostedCluster.destroy),
	)
}

// Install the Hypershift HostedCluster API in the local development cluster.
func (dev *Dev) InstallHypershiftAPIs(ctx context.Context, _ []string) error {
	self := run.Meth1(dev, dev.InstallHypershiftAPIs, []string{})
	if err := mgr.ParallelDeps(ctx, self,
		run.Meth(cluster, cluster.create),
	); err != nil {
		return err
	}

	clClients, err := cluster.Clients()
	if err != nil {
		return fmt.Errorf("getting dev cluster client: %w", err)
	}

	// install hosted cluster CRD into mgmt cluster
	hcCrdPath := filepath.Join("integration", "package-operator", "testdata", "hostedclusters.crd.yaml")

	if err = clClients.CreateAndWaitFromFiles(ctx, []string{hcCrdPath}); err != nil {
		return fmt.Errorf("applying HostedCluster CRD to dev cluster: %w", err)
	}
	return nil
}

// Create the local Hypershift development environment.
func (dev *Dev) CreateHostedCluster(ctx context.Context, args []string) error {
	self := run.Meth1(dev, dev.CreateHostedCluster, args)
	if err := mgr.ParallelDeps(ctx, self,
		run.Meth1(dev, dev.LoadCRDs, []string{}),
		run.Meth1(dev, dev.InstallHypershiftAPIs, []string{}),
		run.Meth(hypershiftHostedCluster, hypershiftHostedCluster.create),
	); err != nil {
		return err
	}

	// get mgmt cluster clients
	clClients, err := cluster.Clients()
	if err != nil {
		return fmt.Errorf("can't get client for mgmt cluster %s: %w", cluster.Name(), err)
	}

	// create package-operator-remote-phase-manager ClusterRole in mgmt cluster
	rpmCrPath := filepath.Join("config", "packages", "package-operator", "rbac",
		"package-operator-remote-phase-manager.ClusterRole.yaml")
	if err = clClients.CreateAndWaitFromFiles(ctx, []string{rpmCrPath}); err != nil {
		return fmt.Errorf("can't create remote phase manager ClusterRole in mgmt cluster %s: %w", cluster.Name(), err)
	}

	// get kubeconfig of hosted cluster and replace hostname with cluster IP
	hostedClKubeconfig, err := hypershiftHostedCluster.Kubeconfig(true)
	if err != nil {
		return fmt.Errorf("can't get Kubeconfig of hosted cluster %s: %w", hypershiftHostedCluster.Name(), err)
	}

	// create namespace
	namespaceName := "default-" + hypershiftHostedCluster.Name()
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
	if err := clClients.CreateAndWaitForReadiness(ctx, namespace); err != nil {
		return fmt.Errorf("can't create hosted cluster namespace in mgmt cluster %s: %w", cluster.Name(), err)
	}

	// create secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service-network-admin-kubeconfig",
			Namespace: namespaceName,
		},
		Data: map[string][]byte{
			"kubeconfig": []byte(hostedClKubeconfig),
		},
	}
	if err := clClients.CreateAndWaitForReadiness(ctx, secret); err != nil {
		return fmt.Errorf("can't create kubeconfig secret in mgmt cluster %s: %w", cluster.Name(), err)
	}

	// create hosted cluster
	hostedClResource := &hsv1beta1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hypershiftHostedCluster.Name(),
			Namespace: "default",
		},
	}

	if err := clClients.CreateAndWaitForReadiness(ctx, hostedClResource); err != nil {
		return fmt.Errorf("can't create HostedCluster in mgmt cluster %s: %w", cluster.Name(), err)
	}

	hostedClResource.Status.Conditions = []metav1.Condition{
		{
			Type:               hsv1beta1.HostedClusterAvailable,
			Status:             metav1.ConditionTrue,
			Reason:             "Success",
			Message:            "HostedCluster is Available (manually set)",
			ObservedGeneration: hostedClResource.GetGeneration(),
			LastTransitionTime: metav1.Now(),
		},
	}
	if err := clClients.CtrlClient.Status().Update(ctx, hostedClResource); err != nil {
		return fmt.Errorf("can't apply HostedCluster status in mgmt cluster %s: %w", cluster.Name(), err)
	}

	return nil
}

// Run Package Operator manager connected to local development cluster.
func (dev *Dev) Run(ctx context.Context, args []string) error {
	self := run.Meth1(dev, dev.Run, args)
	if err := mgr.SerialDeps(ctx, self,
		run.Meth1(dev, dev.LoadCRDs, []string{}),
		run.Fn(func() error {
			// get mgmt cluster clients
			clClients, err := cluster.Clients()
			if err != nil {
				return fmt.Errorf("can't get client for cluster %s: %w", cluster.Name(), err)
			}
			if err := clClients.CtrlClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "package-operator-system",
				},
			}); err != nil && !apimachineryerrors.IsAlreadyExists(err) {
				return err
			}
			return nil
		}),
	); err != nil {
		return err
	}

	goBinPath, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("looking up go binary from PATH: %w", err)
	}
	absGoBinPath, err := filepath.Abs(goBinPath)
	if err != nil {
		return fmt.Errorf("resolving absolute go binary path: %w", err)
	}
	kubeconfigPath, err := cluster.KubeconfigPath()
	if err != nil {
		return fmt.Errorf("retrieving cluster kubeconfig path: %w", err)
	}

	if err := os.Setenv("KUBECONFIG", kubeconfigPath); err != nil {
		return fmt.Errorf("setting KUBECONFIG env variable: %w", err)
	}

	goArgs := []string{
		absGoBinPath,
		"run",
		"./cmd/package-operator-manager",
		"-namespace", "package-operator-system",
		"-enable-leader-election=true",
		"-registry-host-overrides", "quay.io=localhost:5001",
		"--remote-phase-package-image", imageRegistry() + "/package-operator-package:" + appVersion,
	}

	return unix.Exec(absGoBinPath, goArgs, os.Environ())
}
