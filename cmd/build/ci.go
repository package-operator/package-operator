package main

import (
	"context"
	"errors"

	"pkg.package-operator.run/cardboard/run"
)

// CI targets that should only be called within the CI/CD runners.
type CI struct{}

// Unit runs unittests in CI.
func (ci *CI) Unit(ctx context.Context, _ []string) error {
	return test.Unit(ctx, "")
}

// Integration runs integration tests in CI using a KinD cluster.
func (ci *CI) Integration(ctx context.Context, _ []string) error {
	return test.Integration(ctx, true, "")
}

// Lint runs linters in CI to check the codebase.
func (ci *CI) Lint(_ context.Context, _ []string) error {
	return lint.glciCheck()
}

// PostPush runs autofixes in CI and validates that the repo is clean afterwards.
func (ci *CI) PostPush(ctx context.Context, args []string) error {
	self := run.Meth1(ci, ci.PostPush, args)
	if err := mgr.ParallelDeps(ctx, self,
		run.Meth(generate, generate.All),
		run.Meth(lint, lint.glciFix),
		run.Meth(lint, lint.goModTidyAll),
	); err != nil {
		return err
	}

	return lint.validateGitClean()
}

// Release builds binaries and releases the CLI, PKO manager, PKO webhooks and test-stub images to the given registry.
func (ci *CI) Release(ctx context.Context, args []string) error {
	registry := imageRegistry()

	if len(args) > 2 {
		return errors.New("target registry as a single arg or no args for official") //nolint:goerr113
	} else if len(args) == 1 {
		registry = args[1]
	}
	if registry == "" {
		return errors.New("registry may not be empty") //nolint:goerr113
	}

	self := run.Meth1(ci, ci.Release, args)
	if err := mgr.ParallelDeps(ctx, self,
		// binary images
		run.Fn3(pushImage, "cli", registry, "amd64"),
		run.Fn3(pushImage, "package-operator-manager", registry, "amd64"),
		run.Fn3(pushImage, "package-operator-webhook", registry, "amd64"),
		run.Fn3(pushImage, "remote-phase-manager", registry, "amd64"),
		run.Fn3(pushImage, "test-stub", registry, "amd64"),

		// package images
		run.Fn2(pushPackage, "remote-phase", registry),
		run.Fn2(pushPackage, "test-stub", registry),
		run.Fn2(pushPackage, "test-stub-multi", registry),
	); err != nil {
		return err
	}

	// This needs to be separate because the remote-phase package image has to be pushed before
	// downstream dependencies of the package-operator package image can be regenerated.
	// *very very sad @erdii noises*
	return mgr.ParallelDeps(ctx, self,
		run.Fn2(pushPackage, "package-operator", registry),
	)
}
