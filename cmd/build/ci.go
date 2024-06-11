package main

import (
	"context"

	"pkg.package-operator.run/cardboard/run"
)

const (
	releaseArgImagesOnly   = "images-only"
	releaseArgValidateFIPS = "validate-fips"
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

// Expose crane login to CI.
func (ci *CI) RegistryLogin(_ context.Context, args []string) error {
	return shr.Run("crane", append([]string{"auth", "login"}, args...)...)
}

func hasArg(args []string, arg string) bool {
	for _, a := range args {
		if a == arg {
			return true
		}
	}
	return false
}

// Release builds binaries (if not exluded with the 'images-only" arg),
// validates their FIPS compliance (when requested with the 'validate-fips' arg)
// and releases the CLI, PKO manager, RP manager, PKO webhooks and test-stub images to the given registry.
func (ci *CI) Release(ctx context.Context, args []string) error {
	registry := imageRegistry()

	self := run.Meth1(ci, ci.Release, args)

	deps := []run.Dependency{}

	imagesOnly := hasArg(args, releaseArgImagesOnly)
	if !imagesOnly {
		deps = append(deps,
			// bootstrap job manifests
			run.Meth(generate, generate.selfBootstrapJob),
			// binaries
			run.Fn3(compile, "kubectl-package", "linux", "amd64"),
			run.Fn3(compile, "kubectl-package", "linux", "arm64"),
			run.Fn3(compile, "kubectl-package", "darwin", "amd64"),
			run.Fn3(compile, "kubectl-package", "darwin", "arm64"),
		)
	}

	validate := hasArg(args, releaseArgValidateFIPS)
	deps = append(deps,
		// binary images
		run.Fn4(pushImage, "cli", registry, "amd64", validate),
		run.Fn4(pushImage, "package-operator-manager", registry, "amd64", validate),
		run.Fn4(pushImage, "package-operator-webhook", registry, "amd64", validate),
		run.Fn4(pushImage, "remote-phase-manager", registry, "amd64", validate),
		run.Fn4(pushImage, "test-stub", registry, "amd64", validate),
	)

	if err := mgr.ParallelDeps(ctx, self, deps...); err != nil {
		return err
	}

	return mgr.ParallelDeps(ctx, self,
		// Package images have to be built after binary images have been
		// because the package lockfiles have to be generated from the image manifest hashes
		// and these are only known after pushing to the target registry.
		run.Fn2(pushPackage, "test-stub", registry),
		run.Fn2(pushPackage, "test-stub-multi", registry),
		run.Fn2(pushPackage, "test-stub-cel", registry),
		run.Fn2(pushPackage, "package-operator", registry),
	)
}

// Combined RegistryLogin and Release with images-only arg. (This is our downstream CI target.)
// Validates fips compliance of binaries before releasing them.
func (ci *CI) ReleaseDownstream(ctx context.Context, args []string) error {
	self := run.Meth1(ci, ci.ReleaseDownstream, args)
	return mgr.SerialDeps(ctx, self,
		run.Meth1(ci, ci.RegistryLogin, args),
		run.Meth1(ci, ci.Release, []string{
			releaseArgImagesOnly,
			releaseArgValidateFIPS,
		}),
	)
}
