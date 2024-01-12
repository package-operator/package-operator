package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pkg.package-operator.run/cardboard/sh"
)

func compile(ctx context.Context, cmd string, goos, goarch string) error {
	if err := (Generate{}).all(ctx); err != nil {
		return err
	}

	env := sh.WithEnvironment{
		"CGO_ENABLED": "0",
		"GOOS":        goos,
		"GOARCH":      goarch,
	}

	if cgo, cgoOk := os.LookupEnv("CGO_ENABLED"); cgoOk {
		env["CGO_ENABLED"] = cgo
	}
	if goos == "" || goarch == "" {
		return fmt.Errorf("invalid os or arch") //nolint:goerr113
	}

	dst := filepath.Join("bin", fmt.Sprintf("%s_%s_%s", cmd, goos, goarch))

	ldflags := []string{
		"-w", "-s", "--extldflags", "'-zrelro -znow -O1'",
		"-X", fmt.Sprintf("'package-operator.run/internal/version.version=%s'", appVersion),
	}

	err := shr.New(env).Run(
		"go", "build", "--ldflags", strings.Join(ldflags, " "), "--trimpath", "--mod=readonly", "-o", dst, "./cmd/"+cmd,
	)
	if err != nil {
		panic(fmt.Errorf("compiling cmd/%s: %w", cmd, err))
	}

	return nil
}

func compileAll(ctx context.Context) error {
	if err := compile(ctx, "package-operator-manager", "linux", "amd64"); err != nil {
		return err
	}
	if err := compile(ctx, "remote-phase-manager", "linux", "amd64"); err != nil {
		return err
	}
	if err := compile(ctx, "remote-phase-manager", "linux", "amd64"); err != nil {
		return err
	}
	if err := compile(ctx, "kubectl-package", "linux", "amd64"); err != nil {
		return err
	}
	if err := compile(ctx, "kubectl-package", "darwin", "amd64"); err != nil {
		return err
	}
	if err := compile(ctx, "kubectl-package", "darwin", "arm64"); err != nil {
		return err
	}

	return nil
}
