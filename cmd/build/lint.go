package main

import (
	"context"

	"pkg.package-operator.run/cardboard/run"
	"pkg.package-operator.run/cardboard/sh"
)

// Lint is a collection of lint related functions.
type Lint struct{}

func (l Lint) goModTidy(workdir string) error {
	return shr.New(sh.WithWorkDir(workdir)).Run("go", "mod", "tidy")
}

func (l Lint) goModTidyAll(ctx context.Context) error {
	return mgr.ParallelDeps(ctx, run.Meth(l, l.goModTidyAll),
		run.Meth1(l, l.goModTidy, "."),
		run.Meth1(l, l.goModTidy, "./apis/"),
	)
}

func (Lint) glciFix() error {
	return shr.Run("golangci-lint", "run", "--timeout=3m",
		"--build-tags=integration,integration_hypershift", "--fix",
		"./...", "./apis/...")
}

func (Lint) glciCheck() error {
	return shr.Run("golangci-lint", "run", "--timeout=3m",
		"--build-tags=integration,integration_hypershift",
		"./...", "./apis/...")
}

func (Lint) govulnCheck() error {
	return shr.Run("govulncheck", "--show=verbose", "./...")
}

func (Lint) validateGitClean() error {
	return shr.Run("git", "diff", "--exit-code")
}

func (Lint) goWorkSync() error {
	return shr.Run("go", "work", "sync")
}
