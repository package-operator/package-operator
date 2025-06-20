package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pkg.package-operator.run/cardboard/run"
	"pkg.package-operator.run/cardboard/sh"
)

type Compile struct{}

// compiles code in /cmd/<cmd> for the given OS and ARCH.
// Binaries will be put in /bin/<cmd>_<os>_<arch>.
func (c Compile) compile(ctx context.Context, cmd string, goos, goarch string) error {
	self := run.Meth3(c, c.compile, cmd, goos, goarch)
	err := mgr.SerialDeps(ctx,
		self,
		run.Meth(generate, generate.All),
	)
	if err != nil {
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
		return errors.New("invalid os or arch")
	}

	dst := filepath.Join("bin", fmt.Sprintf("%s_%s_%s", cmd, goos, goarch))

	ldflags := []string{
		"-w", "-buildid=",
		"--extldflags", "'-zrelro -znow -O1'",
		"-X", fmt.Sprintf("'package-operator.run/internal/version.version=%s'", appVersion),
	}

	err = shr.New(env).Run(
		"go", "build", "--ldflags", strings.Join(ldflags, " "), "--trimpath", "--mod=readonly", "-o", dst, "./cmd/"+cmd,
	)
	if err != nil {
		return fmt.Errorf("compiling cmd/%s: %w", cmd, err)
	}

	return nil
}
