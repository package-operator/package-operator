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
	if err := mgr.SerialDeps(ctx, self,
		run.Meth(generate, generate.All),
		run.Meth(lint, lint.glciFix),
		run.Meth(lint, lint.goWorkSync),
		run.Meth(lint, lint.goModTidyAll),
	); err != nil {
		return err
	}

	return lint.validateGitClean()
}

// Expose crane login to CI.
func (ci *CI) RegistryLogin(_ context.Context, args []string) error {
	return shr.Run("crane", append([]string{"auth", "login"}, args...)...)
}

// Release builds binaries and helm chart (if not exluded with the 'images-only" arg) and releases the
// CLI, PKO manager, RP manager, and test-stub images to the given registry.
func (ci *CI) Release(ctx context.Context, args []string) error {
	self := run.Meth1(ci, ci.Release, args)

	registry := imageRegistry()

	deps := []run.Dependency{}

	imagesOnly := len(args) > 0 && args[0] == "images-only"
	if !imagesOnly {
		deps = append(deps,
			// bootstrap job manifests
			run.Meth(generate, generate.selfBootstrapJob),
			// binaries
			run.Meth3(compile, compile.compile, "kubectl-package", "linux", "amd64"),
			run.Meth3(compile, compile.compile, "kubectl-package", "linux", "arm64"),
			run.Meth3(compile, compile.compile, "kubectl-package", "darwin", "amd64"),
			run.Meth3(compile, compile.compile, "kubectl-package", "darwin", "arm64"),
			// helm chart
			run.Meth1(chart, chart.push, []string{}),
		)
	}

	deps = append(deps,
		// binary images
		run.Fn3(pushImage, "cli", registry, "amd64"),
		run.Fn3(pushImage, "package-operator-manager", registry, "amd64"),
		run.Fn3(pushImage, "remote-phase-manager", registry, "amd64"),
	)

	if err := mgr.ParallelDeps(ctx, self, deps...); err != nil {
		return err
	}

	return mgr.ParallelDeps(ctx, self,
		// Package images have to be built after binary images have been
		// because the package lockfiles have to be generated from the image manifest hashes
		// and these are only known after pushing to the target registry.
		run.Fn2(pushPackage, "package-operator", registry),
	)
}

// Combined RegistryLogin and Release with images-only arg. (This is our downstream CI target.)
func (ci *CI) RegistryLoginAndReleaseOnlyImages(ctx context.Context, args []string) error {
	self := run.Meth1(ci, ci.RegistryLoginAndReleaseOnlyImages, args)
	return mgr.SerialDeps(ctx, self,
		run.Meth1(ci, ci.RegistryLogin, args),
		run.Meth1(ci, ci.Release, []string{"images-only"}),
	)
}

var errInvalidArguments = errors.New("invalid number of arguments, usage: ./do CI:Compile <cmd> <os> <arch>")

// Compiles code in /cmd/<cmd> for the given OS and ARCH. Binaries will be put in /bin/<cmd>_<os>_<arch>.
func (ci *CI) Compile(ctx context.Context, args []string) error {
	if len(args) < 3 {
		return errInvalidArguments
	}
	return compile.compile(ctx, args[0], args[1], args[2])
}

// Runs govulncheck.
func (ci *CI) GovulnCheck(_ context.Context, _ []string) error {
	return lint.govulnCheck()
}
