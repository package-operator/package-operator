package main

import (
	"context"

	"pkg.package-operator.run/cardboard/run"
	"pkg.package-operator.run/cardboard/sh"
)

// internal struct to namespace all lint related functions.
type Lint struct{}

func (l Lint) Fix() error   { return l.glciFix() }
func (l Lint) Check() error { return l.glciCheck() }

// TODO make that more nice.
func (l Lint) goModTidy(workdir string) error {
	return shr.New(sh.WithWorkDir(workdir)).Run("go", "mod", "tidy")
}

func (l Lint) goModTidyAll(ctx context.Context) error {
	return mgr.ParallelDeps(ctx, run.Meth(l, l.goModTidyAll),
		run.Meth1(l, l.goModTidy, "."),
		run.Meth1(l, l.goModTidy, "./apis/"),
		run.Meth1(l, l.goModTidy, "./pkg/"),
	)
}

func (Lint) glciFix() error {
	return shr.Run("golangci-lint", "run", "--fix", "--deadline=15m", "./...", "./apis/...", "./pkg/...")
}

func (Lint) glciCheck() error {
	return shr.Run("golangci-lint", "run", "--deadline=15m", "./...", "./apis/...", "./pkg/...")
}
