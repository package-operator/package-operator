package main

import (
	"context"
	"fmt"

	"pkg.package-operator.run/cardboard/run"
)

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
