package main

import (
	"context"
	"os"
	"path/filepath"

	"package-operator.run/internal/cmd"

	"pkg.package-operator.run/cardboard/modules/oci"
)

func buildPackage(ctx context.Context, name, registry string) error {
	if err := os.MkdirAll(filepath.Join(".cache", "packages"), 0o755); err != nil {
		return err
	}

	switch name {
	case "remote-phase":
		if err := (Generate{}).remotePhaseFiles(ctx); err != nil {
			return err
		}
	case "package-operator":
		if err := (Generate{}).packageOperatorPackageFiles(ctx); err != nil {
			return err
		}
	}

	path := filepath.Join("config", "packages", name, "container.oci.tar")
	err := cmd.NewBuild().BuildFromSource(ctx,
		filepath.Join("config", "packages", name),
		cmd.WithOutputPath(path),
		cmd.WithTags{registry + "/" + name + "-package:" + appVersion},
	)
	if err != nil {
		return err
	}

	return oci.NewOCI("", "").Load(path)
}

func pushPackage(ctx context.Context, name, registry string) error {
	imgPath, err := filepath.Abs(filepath.Join("config", "packages", name))
	if err != nil {
		return err
	}

	if err := buildPackage(ctx, name, registry); err != nil {
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
