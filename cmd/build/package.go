package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"package-operator.run/internal/cmd"

	"pkg.package-operator.run/cardboard/modules/oci"
	"pkg.package-operator.run/cardboard/run"
)

func buildPackage(ctx context.Context, name, registry string) error {
	if err := os.MkdirAll(filepath.Join(cacheDir, "packages"), 0o755); err != nil {
		return err
	}

	deps := []run.Dependency{}

	if name == "package-operator" {
		deps = append(deps,
			run.Meth(generate, generate.remotePhaseComponentFiles),
			run.Meth(generate, generate.hostedClusterComponentFiles),
			run.Meth(generate, generate.packageOperatorPackageFiles),
		)
	} else {
		deps = append(deps, run.Meth1(generate, generate.templateManifestFiles, name))
	}

	self := run.Fn2(buildPackage, name, registry)
	if err := mgr.SerialDeps(
		ctx, self,
		deps...,
	); err != nil {
		return err
	}

	manifestDir := filepath.Join("config", "packages", name)
	outDir := filepath.Join(".cache", "packages", name)
	path := filepath.Join(outDir, "container.oci.tar")
	url := imageURL(registry, name+"-package", appVersion)

	// Prepare output directory because kubectl-package (the `cmd` below) expects that it exists.
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("making directory tree: %w", err)
	}

	if err := cmd.NewBuild().BuildFromSource(ctx,
		manifestDir,
		cmd.WithOutputPath(path),
		cmd.WithTags{url},
	); err != nil {
		return err
	}

	return oci.NewOCI("", "").Load(ctx, path)
}

func pushPackage(ctx context.Context, name, registry string) error {
	self := run.Fn2(pushPackage, name, registry)
	if err := mgr.SerialDeps(
		ctx, self,
		run.Fn2(buildPackage, name, registry),
	); err != nil {
		return err
	}

	imgPath, err := filepath.Abs(filepath.Join(".cache", "packages", name))
	if err != nil {
		return err
	}

	url := imageURL(registry, name+"-package", appVersion)

	if err := oci.NewOCI(url, imgPath, oci.WithCranePush{}).Push(ctx); err != nil {
		return err
	}

	return nil
}
