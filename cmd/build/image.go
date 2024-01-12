package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"pkg.package-operator.run/cardboard/modules/oci"
	"pkg.package-operator.run/cardboard/run"
	"pkg.package-operator.run/cardboard/sh"
)

func buildImage(ctx context.Context, name, registry string) error {
	buildDir, err := filepath.Abs(".cache/images/" + name + "/")
	if err != nil {
		return err
	}
	if err := os.RemoveAll(buildDir); err != nil {
		return err
	}
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return err
	}

	binaryName := name
	if name == "cli" {
		binaryName = "kubectl-package"
	}
	if err := compile(ctx, binaryName, "linux", "amd64"); err != nil {
		return err
	}

	if err := shr.Copy(buildDir+"/"+binaryName, "./bin/"+binaryName+"_linux_amd64"); err != nil {
		return err
	}

	o := oci.NewOCI(name, buildDir,
		oci.WithContainerFile("Containerfile"),
		oci.WithTags{appVersion},
		oci.WithRegistries{registry},
	)

	if err := shr.Copy(buildDir+"/passwd", "config/images/passwd"); err != nil {
		return err
	}
	if err := shr.Copy(buildDir+"/Containerfile", "config/images/"+name+".Containerfile"); err != nil {
		return err
	}

	if err := compileAll(ctx); err != nil {
		return err
	}

	if err := o.Build(); err != nil {
		return err
	}

	return nil
}

func pushImage(ctx context.Context, name, registry string) error {
	imgPath, err := filepath.Abs("./config/images/")
	if err != nil {
		return err
	}

	if err := buildImage(ctx, name, registry); err != nil {
		return err
	}
	o := oci.NewOCI(name, imgPath,
		oci.WithTags{appVersion},
		oci.WithRegistries{registry},
	)

	if err := o.Push(); err != nil {
		return err
	}

	return nil
}

func imageURL(registry, name, version string) string {
	url := os.Getenv(strings.ReplaceAll(strings.ToUpper(name), "-", "_") + "_IMAGE")
	if url == "" {
		url = fmt.Sprintf("%s/%s:%s", registry, name, version)
	}

	return url
}

func version() (string, error) {
	// Use version from VERSION env if present, use "git describe" elsewise.
	if pkoVersion := strings.TrimSpace(os.Getenv("VERSION")); pkoVersion != "" {
		return pkoVersion, nil
	}

	version, err := shr.New(sh.WithLogger{}).Output("git", "describe", "--tags")
	if err != nil {
		return "", fmt.Errorf("git describe: %w", err)
	}

	version = strings.Split(version, "-")[0]

	// Depending on what process was used the last tag my either be a version for
	// the main module (eg `v1.6.6`) or a version for a submodule (eg `apis/v1.6.6`).
	return path.Base(strings.TrimSpace(version)), nil
}

func mustVersion() string {
	v, err := version()
	if err != nil {
		run.Must(err)
	}

	return v
}
