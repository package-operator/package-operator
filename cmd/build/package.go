package main

import (
	"context"
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
	}

	self := run.Fn2(buildPackage, name, registry)
	if err := mgr.ParallelDeps(
		ctx, self,
		deps...,
	); err != nil {
		return err
	}

	path := filepath.Join("config", "packages", name, "container.oci.tar")
	url := imageURL(registry, name+"-package", appVersion)

	if err := cmd.NewBuild().BuildFromSource(ctx,
		filepath.Join("config", "packages", name),
		cmd.WithOutputPath(path),
		cmd.WithTags{url},
	); err != nil {
		return err
	}

	return oci.NewOCI("", "").Load(path)
}

func pushPackage(ctx context.Context, name, registry string) error {
	self := run.Fn2(buildPackage, name, registry)
	if err := mgr.SerialDeps(
		ctx, self,
		run.Fn2(buildPackage, name, registry),
	); err != nil {
		return err
	}

	imgPath, err := filepath.Abs(filepath.Join("config", "packages", name))
	if err != nil {
		return err
	}

	url := imageURL(registry, name+"-package", appVersion)

	if err := oci.NewOCI(url, imgPath, oci.WithCranePush{}).Push(); err != nil {
		return err
	}

	return nil
}
