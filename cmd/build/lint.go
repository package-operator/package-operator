package main

import (
	"context"

	"pkg.package-operator.run/cardboard/run"
	"pkg.package-operator.run/cardboard/sh"
)

// Lint is a collection of lint related functions.
type Lint struct{}

func (l Lint) goModTidy(ctx context.Context, workdir string) error {
	return shr.New(sh.WithWorkDir(workdir)).Run(ctx, "go", "mod", "tidy")
}

func (l Lint) goModTidyAll(ctx context.Context) error {
	return mgr.ParallelDeps(ctx, run.Meth(l, l.goModTidyAll),
		run.Meth1(l, l.goModTidy, "."),
		run.Meth1(l, l.goModTidy, "./apis/"),
	)
}

func (Lint) glciFix(ctx context.Context) error {
	return shr.Run(ctx, "golangci-lint", "run", "--timeout=3m",
		"--build-tags=integration,integration_hypershift", "--fix",
		"./...", "./apis/...")
}

func (Lint) glciCheck(ctx context.Context) error {
	return shr.Run(ctx, "golangci-lint", "run", "--timeout=3m",
		"--build-tags=integration,integration_hypershift",
		"./...", "./apis/...")
}

func (Lint) govulnCheck(ctx context.Context) error {
	return shr.Run(ctx, "govulncheck", "--show=verbose", "./...")
}

func (Lint) validateGitClean(ctx context.Context) error {
	return shr.Run(ctx, "git", "diff", "--exit-code")
}

func (Lint) goWorkSync(ctx context.Context) error {
	return shr.Run(ctx, "go", "work", "sync")
}
