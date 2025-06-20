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
)

// Dev focused commands using local development environment.
type Dev struct{}

// PrintVersion prints app version.
func (dev *Dev) PrintVersion(_ context.Context, _ []string) error {
	fmt.Println(appVersion) //nolint:forbidigo
	return nil
}

// PreCommit runs linters and code-gens for pre-commit.
func (dev *Dev) PreCommit(ctx context.Context, args []string) error {
	self := run.Meth1(dev, dev.PreCommit, args)

	if err := mgr.SerialDeps(ctx, self,
		run.Meth(generate, generate.All),
	); err != nil {
		return err
	}

	return mgr.SerialDeps(ctx, self,
		run.Meth(lint, lint.glciFix),
		run.Meth(lint, lint.goWorkSync),
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
		return errors.New("only supports a single argument")
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
		return errors.New("only supports a single argument")
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

// Runs govulncheck against the code in this repo.
func (Dev) Govulncheck(_ context.Context, _ []string) error {
	return lint.govulnCheck()
}

// Create the local development cluster.
func (dev *Dev) Create(ctx context.Context, _ []string) error {
	return cluster.create(ctx)
}

// Bootstrap package-operator-manager in local development cluster.
func (dev *Dev) Bootstrap(ctx context.Context, _ []string) error {
	return bootstrap(ctx)
}

// Load CRDs into the local development cluster.
func (dev *Dev) LoadCRDs(ctx context.Context, args []string) error {
	return cluster.loadCRDs(ctx, args)
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
func (dev *Dev) InstallHypershiftAPIs(ctx context.Context, args []string) error {
	return cluster.installHypershiftAPIs(ctx, args)
}

// Create the local Hypershift development environment.
func (dev *Dev) CreateHostedCluster(ctx context.Context, args []string) error {
	return hypershiftHostedCluster.createHostedCluster(ctx, &cluster, args)
}

// Run Package Operator manager connected to local development cluster.
func (dev *Dev) Run(ctx context.Context, args []string) error {
	self := run.Meth1(dev, dev.Run, args)
	if err := mgr.SerialDeps(ctx, self,
		run.Meth1(cluster, cluster.loadCRDs, []string{}),
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
		"-service-account-namespace", "default",
		"-service-account-name", "default",
		"-registry-host-overrides", imageRegistryHost() + "=localhost:5001",
		"--package-operator-package-image", imageRegistry() + "/package-operator-package:" + appVersion,
	}
	goArgs = append(goArgs, args...)

	return unix.Exec(absGoBinPath, goArgs, os.Environ())
}
