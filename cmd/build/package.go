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

	switch name {
	case "remote-phase":
		deps = append(deps, run.Meth(generate, generate.remotePhaseFiles))
	case "package-operator":
		deps = append(deps, run.Meth(generate, generate.packageOperatorPackageFiles))
	}

	self := run.Fn2(buildPackage, name, registry)
	if err := mgr.ParallelDeps(
		ctx, self,
		deps...,
	); err != nil {
		return err
	}

	path := filepath.Join("config", "packages", name, "container.oci.tar")

	if err := cmd.NewBuild().BuildFromSource(ctx,
		filepath.Join("config", "packages", name),
		cmd.WithOutputPath(path),
		cmd.WithTags{registry + "/" + name + "-package:" + appVersion},
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

	o := oci.NewOCI(name+"-package", imgPath,
		oci.WithTags{appVersion},
		oci.WithRegistries{registry},
		oci.WithCranePush{},
	)

	if err := o.Push(); err != nil {
		return err
	}

	return nil
}
