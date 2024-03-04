package main

import (
	"context"
	"errors"

	"pkg.package-operator.run/cardboard/run"
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

// Destroy the local development cluster.
func (dev *Dev) Destroy(ctx context.Context, _ []string) error {
	return cluster.destroy(ctx)
}
