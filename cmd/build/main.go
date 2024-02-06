package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"pkg.package-operator.run/cardboard/modules/kind"
	"pkg.package-operator.run/cardboard/modules/kubeclients"
	"pkg.package-operator.run/cardboard/run"
	"pkg.package-operator.run/cardboard/sh"
	kindv1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

var (
	shr        *sh.Runner
	mgr        *run.Manager
	appVersion string

	// internal modules.
	generate *Generate
	test     *Test
	lint     *Lint
	cluster  *Cluster

	//go:embed *.go
	source embed.FS
)

func main() {
	ctx := context.Background()

	mgr = run.New(run.WithSources(source))
	shr = sh.New()
	clusterCfg := kindv1alpha4.Cluster{
		ContainerdConfigPatches: []string{
			// Replace `imageRegistry` with our local dev-registry.
			fmt.Sprintf(`[plugins."io.containerd.grpc.v1.cri".registry.mirrors."%s"]
endpoint = ["http://localhost:31320"]`, "quay.io"),
		},
		Nodes: []kindv1alpha4.Node{
			{
				Role: kindv1alpha4.ControlPlaneRole,
				ExtraPortMappings: []kindv1alpha4.PortMapping{
					// Open port to enable connectivity with local registry.
					{
						ContainerPort: 5001,
						HostPort:      5001,
						ListenAddress: "127.0.0.1",
						Protocol:      "TCP",
					},
				},
			},
		},
	}
	generate = &Generate{}
	test = &Test{}
	lint = &Lint{}
	cluster = &Cluster{
		kind.NewCluster("pko",
			kind.WithClusterConfig(clusterCfg),
			kind.WithClientOptions{
				kubeclients.WithSchemeBuilder{corev1alpha1.AddToScheme},
			},
			kind.WithClusterInitializers{
				kind.ClusterLoadObjectsFromFiles{filepath.Join("config", "local-registry.yaml")},
			},
		),
	}

	appVersion = mustVersion()

	err := errors.Join(
		// Required by cardboard itself.
		mgr.RegisterGoTool("crane", "github.com/google/go-containerregistry/cmd/crane", "0.16.1"),
		mgr.RegisterGoTool("kind", "sigs.k8s.io/kind", "0.20.0"),
		// Our deps
		mgr.RegisterGoTool("gotestfmt", "github.com/gotesttools/gotestfmt/v2/cmd/gotestfmt", "2.5.0"),
		mgr.RegisterGoTool("controller-gen", "sigs.k8s.io/controller-tools/cmd/controller-gen", "0.13.0"),
		mgr.RegisterGoTool("conversion-gen", "k8s.io/code-generator/cmd/conversion-gen", "0.28.3"),
		mgr.RegisterGoTool("golangci-lint", "github.com/golangci/golangci-lint/cmd/golangci-lint", "1.55.0"),
		mgr.RegisterGoTool("k8s-docgen", "github.com/thetechnick/k8s-docgen", "0.6.2"),
		mgr.RegisterGoTool("helm", "helm.sh/helm/v3/cmd/helm", "3.12.3"),
		mgr.Register(&Dev{}, &CI{}),
	)
	if err != nil {
		panic(err)
	}

	if err := mgr.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s\n", err)
		os.Exit(1)
	}
}

//
// Callable targets defined below.
//

// CI targets that should only be called within the CI/CD runners.
type CI struct{}

// Runs unittests in CI.
func (ci *CI) Unit(ctx context.Context, _ []string) error {
	return test.Unit(ctx, "")
}

// Runs integration tests in CI using a KinD cluster.
func (ci *CI) Integration(ctx context.Context, _ []string) error {
	return test.Integration(ctx, "")
}

// Runs linters in CI to check the codebase.
func (ci *CI) Lint(_ context.Context, _ []string) error {
	return lint.check()
}

func (ci *CI) PostPush(ctx context.Context, args []string) error {
	self := run.Meth1(ci, ci.PostPush, args)
	err := mgr.ParallelDeps(ctx, self,
		run.Meth(generate, generate.All),
		run.Meth(lint, lint.glciFix),
		run.Meth(lint, lint.goModTidyAll),
	)
	if err != nil {
		return err
	}

	return shr.Run("git", "diff", "--quiet", "--exit-code")
}

// Builds binaries and releases the CLI, PKO manager, PKO webhooks and test-stub images to the given registry.
func (ci *CI) Release(ctx context.Context, args []string) error {
	self := run.Meth1(ci, ci.Release, args)
	registry := "quay.io/package-operator"
	if len(args) > 2 {
		return fmt.Errorf("traget registry as a single arg or no args for official") //nolint:goerr113
	} else if len(args) == 1 {
		registry = args[1]
	}

	if registry == "" {
		return fmt.Errorf("registry may not be empty") //nolint:goerr113
	}

	return mgr.ParallelDeps(ctx, self,
		run.Fn2(pushImage, "cli", registry),
		run.Fn2(pushImage, "package-operator-manager", registry),
		run.Fn2(pushImage, "package-operator-webhook", registry),
		run.Fn2(pushImage, "remote-phase-manager", registry),
		run.Fn2(pushImage, "test-stub", registry),
	)
}

// Development focused commands using local development environment.
type Dev struct{}

// Runs linters and code-gens for pre-commit.
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
	if err := mgr.SerialDeps(
		ctx, self,
		run.Meth(generate, generate.code),
	); err != nil {
		return err
	}

	// installYamlFile has to come after code generation.
	return mgr.ParallelDeps(
		ctx, self,
		run.Meth(generate, generate.docs),
		run.Meth(generate, generate.installYamlFile),
		run.Meth(generate, generate.selfBootstrapJob),
		run.Meth(generate, generate.selfBootstrapJobLocal),
	)
}

// Runs local unittests.
func (dev *Dev) Unit(ctx context.Context, args []string) error {
	var filter string
	switch len(args) {
	case 0:
		// nothing
	case 1:
		filter = args[0]
	default:
		return fmt.Errorf("only supports a single argument") //nolint:goerr113
	}
	return test.Unit(ctx, filter)
}

// Runs local integration tests in a KinD cluster.
func (dev *Dev) Integration(ctx context.Context, args []string) error {
	var filter string
	switch len(args) {
	case 0:
		// nothing
	case 1:
		filter = args[0]
	default:
		return fmt.Errorf("only supports a single argument") //nolint:goerr113
	}
	return test.Integration(ctx, filter)
}

// Runs local linters to check the codebase.
func (dev *Dev) Lint(_ context.Context, _ []string) error {
	return lint.check()
}

// Tries to fix linter issues.
func (dev *Dev) LintFix(_ context.Context, _ []string) error {
	return lint.fix()
}

// Deletes the local development cluster.
func (dev *Dev) Create(ctx context.Context, _ []string) error {
	return cluster.create(ctx)
}

// Deletes the local development cluster.
func (dev *Dev) Destroy(ctx context.Context, _ []string) error {
	return cluster.destroy(ctx)
}
