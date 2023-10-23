//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

type Build mg.Namespace

// Build all PKO binaries for the architecture of this machine.
func (Build) Binaries() {
	targets := []any{mg.F(Build.Binary, "mage", "", "")}
	for name := range commands {
		targets = append(targets, mg.F(Build.Binary, name, nativeArch.OS, nativeArch.Arch))
	}

	mg.Deps(targets...)
}

func (Build) ReleaseBinaries() {
	targets := []any{}
	for name, cmd := range commands {
		for _, arch := range cmd.ReleaseArchitectures {
			targets = append(targets, mg.F(Build.Binary, name, arch.OS, arch.Arch))
		}
	}
	mg.Deps(targets...)

	for name, cmd := range commands {
		for _, arch := range cmd.ReleaseArchitectures {
			dst := filepath.Join("bin", fmt.Sprintf("%s_%s_%s", name, arch.OS, arch.Arch))
			must(sh.Copy(dst, locations.binaryDst(name, arch)))
		}
	}
}

// Builds binaries from /cmd directory.
func (Build) Binary(cmd string, goos, goarch string) {
	env := map[string]string{}
	_, cgoOK := os.LookupEnv("CGO_ENABLED")
	if !cgoOK {
		env["CGO_ENABLED"] = "0"
	}

	bin := locations.binaryDst(cmd, nativeArch)
	if len(goos) != 0 || len(goarch) != 0 {
		bin = locations.binaryDst(cmd, archTarget{goos, goarch})
		env["GOOS"] = goos
		env["GOARCH"] = goarch
	}

	ldflags := "-w -s --extldflags '-zrelro -znow -O1'" + fmt.Sprintf("-X '%s/internal/version.version=%s'", module, applicationVersion)
	cmdline := []string{"build", "--ldflags", ldflags, "--trimpath", "--mod=readonly", "-v", "-o", bin, "./cmd/" + cmd}

	if err := sh.RunWithV(env, "go", cmdline...); err != nil {
		panic(fmt.Errorf("compiling cmd/%s: %w", cmd, err))
	}
}

// Builds all PKO container images.
func (Build) Images() {
	deps := []any{}
	for k := range commandImages {
		deps = append(deps, mg.F(Build.Image, k))
	}
	for k := range packageImages {
		deps = append(deps, mg.F(Build.Image, k))
	}
	mg.Deps(deps...)
}

// Builds and pushes only the given container image to the default registry.
func (Build) PushImage(ctx context.Context, imageName string) {
	mg.Deps(mg.F(Build.Image, imageName))
	if pushToDevRegistry {
		mg.Deps(mg.F(Dev.loadImage, imageName))
		return
	}

	cmdOpts, cmdOptsOK := commandImages[imageName]
	pkgOpts, pkgOptsOK := packageImages[imageName]
	switch {
	case cmdOptsOK && pkgOptsOK:
		panic("ambigious image name configured")
	case !cmdOptsOK && !pkgOptsOK:
		panic(fmt.Sprintf("unknown image: %s", imageName))
	case (cmdOptsOK && !cmdOpts.Push) || (pkgOptsOK && !pkgOpts.Push):
		panic(fmt.Sprintf(fmt.Sprintf("image is not configured to be pushed: %s", imageName)))
	}

	must(locations.ContainerRuntime(ctx).PushImage(ctx, locations.ImageURL(imageName, false)))
}

// Builds and pushes all container images to the default registry.
func (Build) PushImages() {
	deps := []any{Generate.SelfBootstrapJob}
	for k, opts := range commandImages {
		if opts.Push {
			deps = append(deps, mg.F(Build.PushImage, k))
		}
	}
	for k, opts := range packageImages {
		if opts.Push {
			deps = append(deps, mg.F(Build.PushImage, k))
		}
	}
	mg.Deps(deps...)
}

// Builds the given container image, building binaries as prerequisite as required.
func (b Build) Image(ctx context.Context, name string) {
	_, isPkg := packageImages[name]
	_, isCmd := commandImages[name]
	switch {
	case isPkg && isCmd:
		panic("ambiguous image name")
	case isPkg:
		b.buildPackageImage(ctx, name)
	case isCmd:
		b.buildCmdImage(ctx, name)
	default:
		panic(fmt.Sprintf("unknown image: %s", name))
	}
}

// clean/prepare cache directory
func (Build) cleanImageCacheDir(name string) {
	imageCacheDir := locations.ImageCache(name)
	if err := os.RemoveAll(imageCacheDir); err != nil && !os.IsNotExist(err) {
		panic(fmt.Errorf("deleting image cache: %w", err))
	}
	if err := os.Remove(imageCacheDir + ".tar"); err != nil && !os.IsNotExist(err) {
		panic(fmt.Errorf("deleting image cache: %w", err))
	}
	if err := os.MkdirAll(imageCacheDir, os.ModePerm); err != nil {
		panic(fmt.Errorf("create image cache dir: %w", err))
	}
}

func (Build) populateCacheCmd(cmd, imageName string) {
	imageCacheDir := locations.ImageCache(imageName)
	must(sh.Copy(filepath.Join(imageCacheDir, cmd), locations.binaryDst(cmd, linuxAMD64Arch)))
	must(sh.Copy(filepath.Join(imageCacheDir, "Containerfile"), filepath.Join("config", "images", imageName+".Containerfile")))
	must(sh.Copy(filepath.Join(imageCacheDir, "passwd"), filepath.Join("config", "images", "passwd")))
}

// generic image build function, when the image just relies on
// a static binary build from cmd/*
func (b Build) buildCmdImage(ctx context.Context, imageName string) {
	opts, ok := commandImages[imageName]
	if !ok {
		panic(fmt.Sprintf("unknown cmd image: %s", imageName))
	}
	cmd := imageName
	if len(opts.BinaryName) != 0 {
		cmd = opts.BinaryName
	}

	mg.Deps(
		mg.F(Build.Binary, cmd, linuxAMD64Arch.OS, linuxAMD64Arch.Arch),
		mg.F(Build.cleanImageCacheDir, imageName),
		mg.F(Build.populateCacheCmd, cmd, imageName),
	)

	must(locations.ContainerRuntime(ctx).BuildImage(ctx, locations.ImageURL(imageName, false), false, ".", "Containerfile"))
}

func (Build) populateCachePkg(imageName, sourcePath string) {
	imageCacheDir := locations.ImageCache(imageName)
	must(sh.Run("cp", "-a", sourcePath+"/.", imageCacheDir+"/"))
}

func mustFilepathAbs(p string) string {
	o, err := filepath.Abs(p)
	must(err)

	return o
}

func newPackageBuildInfo(imageName string) *PackageBuildInfo {
	imageCacheDir := locations.ImageCache(imageName)
	return &PackageBuildInfo{
		ImageTag:       locations.ImageURL(imageName, false),
		CacheDir:       imageCacheDir,
		SourcePath:     imageCacheDir,
		OutputPath:     imageCacheDir + ".tar",
		ExecutablePath: mustFilepathAbs(locations.binaryDst(cliCmdName, nativeArch)),
	}
}

func (b Build) buildPackageImage(ctx context.Context, name string) {
	opts, ok := packageImages[name]
	if !ok {
		panic(fmt.Sprintf("unknown package: %s", name))
	}

	predeps := []any{
		mg.F(Build.Binary, cliCmdName, linuxAMD64Arch.OS, linuxAMD64Arch.Arch),
		mg.F(Build.cleanImageCacheDir, name),
	}
	for _, d := range opts.ExtraDeps {
		predeps = append(predeps, d)
	}
	// populating the cache dir must come LAST, or we might miss generated files.
	predeps = append(predeps, mg.F(Build.populateCachePkg, name, opts.SourcePath))

	mg.Deps(predeps...)

	buildInfo := newPackageBuildInfo(name)
	must(BuildPackage(ctx, buildInfo, predeps))
}

type PackageBuildInfo struct {
	ImageTag string
	CacheDir string
	// source directory
	SourcePath string
	// destination: .tar file path
	OutputPath string
	// will default to "kubectl-package"
	ExecutablePath string
	// if set to `true`, built package won't be loaded into the runtime
	NoRunTimeLoad bool
	// package will be pushed directly using the PKO CLI and not the runtime
	Push bool
}

// BuildPackage builds a package image using the package operator CLI,
// requires `kubectl-package` command to be available on the system
func BuildPackage(ctx context.Context, buildInfo *PackageBuildInfo, deps []interface{}) error {
	if len(deps) > 0 {
		mg.SerialDeps(deps...)
	}

	executable := "kubectl-package"
	if len(buildInfo.ExecutablePath) != 0 {
		executable = buildInfo.ExecutablePath
	}

	buildArgs := []string{
		executable,
		"build", "--tag", buildInfo.ImageTag,
		"--output", buildInfo.OutputPath,
	}
	if buildInfo.Push {
		buildArgs = append(buildArgs, "--push")
	}
	buildArgs = append(buildArgs, buildInfo.SourcePath)

	commands := [][]string{buildArgs}
	if !buildInfo.NoRunTimeLoad {
		commands = append(commands, []string{
			"podman", "load", "--input", buildInfo.OutputPath,
		})
	}

	for _, args := range commands {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Dir = buildInfo.CacheDir

		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}
