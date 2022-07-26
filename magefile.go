//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/mt-sre/devkube/dev"
	"github.com/mt-sre/devkube/magedeps"
	appsv1 "k8s.io/api/apps/v1"
)

const (
	module          = "github.com/package-operator/package-operator"
	defaultImageOrg = "quay.io/nschiede"
)

var (
	// Working directory of the project.
	workDir string
	// Dependency directory.
	depsDir  magedeps.DependencyDirectory
	cacheDir string

	logger           logr.Logger
	containerRuntime string

	// components
	Builder = &builder{}
)

func init() {
	var err error
	// Directories
	workDir, err = os.Getwd()
	if err != nil {
		panic(fmt.Errorf("getting work dir: %w", err))
	}
	cacheDir = path.Join(workDir + "/" + ".cache")
	depsDir = magedeps.DependencyDirectory(path.Join(workDir, ".deps"))
	os.Setenv("PATH", depsDir.Bin()+":"+os.Getenv("PATH"))

	logger = stdr.New(nil)

}

// Testing and Linting
// -------------------
type Test mg.Namespace

func (Test) Lint() error {
	mg.Deps(
		Dependency.GolangciLint,
	)

	for _, cmd := range [][]string{
		{"go", "fmt", "./..."},
		{"bash", "./hack/validate-directory-clean.sh"},
		{"golangci-lint", "run", "./...", "--deadline=15m"},
	} {
		if err := sh.RunV(cmd[0], cmd[1:]...); err != nil {
			return fmt.Errorf("running %q: %w", strings.Join(cmd, " "), err)
		}
	}
	return nil
}

// Runs unittests.
func (Test) Unit() error {
	return sh.RunWithV(map[string]string{
		// needed to enable race detector -race
		"CGO_ENABLED": "1",
	}, "go", "test", "-cover", "-v", "-race", "./internal/...", "./cmd/...")
}

func (Test) Integration() error {
	return sh.Run("go", "test", "-v",
		"-count=1", // will force a new run, instead of using the cache
		"-timeout=20m", "./integration/...")
}

// Building
// --------
type Build mg.Namespace

func (Build) Version() {
	Builder.Version()
}

func (Build) Binaries() {
	mg.Deps(
		mg.F(Builder.Cmd, "package-operator-manager", "linux", "amd64"),
		mg.F(Builder.Cmd, "mage", "", ""),
	)
}

func (Build) Image(image string) {
	mg.Deps(
		mg.F(Builder.Image, image),
	)
}

func (Build) Images() {
	mg.Deps(
		mg.F(Builder.Image, "package-operator-manager"),
	)
}

func (Build) PushImage(image string) {
	mg.Deps(
		mg.F(Builder.Push, image),
	)
}

func (Build) PushImages() {
	mg.Deps(
		mg.F(Builder.Push, "package-operator-manager"),
	)
}

// Dependencies
// ------------

// Dependency Versions
const (
	controllerGenVersion = "0.6.2"
	goimportsVersion     = "0.1.5"
	golangciLintVersion  = "1.46.2"
	kindVersion          = "0.11.1"
)

type Dependency mg.Namespace

func (d Dependency) All() {
	mg.Deps(
		Dependency.ControllerGen,
		Dependency.Goimports,
		Dependency.GolangciLint,
		Dependency.Kind,
	)
}

// Ensure controller-gen - kubebuilder code and manifest generator.
func (d Dependency) ControllerGen() error {
	return depsDir.GoInstall("controller-gen",
		"sigs.k8s.io/controller-tools/cmd/controller-gen", controllerGenVersion)
}

func (d Dependency) Goimports() error {
	return depsDir.GoInstall("go-imports",
		"golang.org/x/tools/cmd/goimports", goimportsVersion)
}

func (d Dependency) GolangciLint() error {
	return depsDir.GoInstall("golangci-lint",
		"github.com/golangci/golangci-lint/cmd/golangci-lint", golangciLintVersion)
}

// Ensure Kind dependency - Kubernetes in Docker (or Podman)
func (d Dependency) Kind() error {
	return depsDir.GoInstall("kind",
		"sigs.k8s.io/kind", kindVersion)
}

// Utility
// -------

// dependency for all targets requiring a container runtime
func determineContainerRuntime() {
	containerRuntime = os.Getenv("CONTAINER_RUNTIME")
	if len(containerRuntime) == 0 || containerRuntime == "auto" {
		cr, err := dev.DetectContainerRuntime()
		if err != nil {
			panic(err)
		}
		containerRuntime = string(cr)
		logger.Info("detected container-runtime", "container-runtime", containerRuntime)
	}
}

// Builder
// -------

type builder struct {
	branch        string
	shortCommitID string
	version       string
	buildDate     string

	// Build Tags
	ldFlags  string
	imageOrg string
}

// init build variables
func (b *builder) init() error {
	// branch
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchBytes, err := branchCmd.Output()
	if err != nil {
		panic(fmt.Errorf("getting git branch: %w", err))
	}
	b.branch = strings.TrimSpace(string(branchBytes))

	// commit id
	shortCommitIDCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	shortCommitIDBytes, err := shortCommitIDCmd.Output()
	if err != nil {
		panic(fmt.Errorf("getting git short commit id"))
	}
	b.shortCommitID = strings.TrimSpace(string(shortCommitIDBytes))

	// version
	b.version = strings.TrimSpace(os.Getenv("VERSION"))
	if len(b.version) == 0 {
		b.version = b.shortCommitID
	}

	// buildDate
	b.buildDate = fmt.Sprint(time.Now().UTC().Unix())

	// flags
	b.ldFlags = fmt.Sprintf(`-X %s/internal/version.Version=%s`+
		`-X %s/internal/version.Branch=%s`+
		`-X %s/internal/version.Commit=%s`+
		`-X %s/internal/version.BuildDate=%s`,
		module, b.version,
		module, b.branch,
		module, b.shortCommitID,
		module, b.buildDate,
	)

	// image org
	b.imageOrg = os.Getenv("IMAGE_ORG")
	if len(b.imageOrg) == 0 {
		b.imageOrg = defaultImageOrg
	}

	return nil
}

func (b *builder) Version() {
	mg.SerialDeps(
		b.init,
	)

	fmt.Println(b.version)
}

// Builds binaries from /cmd directory.
func (b *builder) Cmd(cmd, goos, goarch string) error {
	mg.SerialDeps(
		b.init,
	)

	env := map[string]string{
		"GOFLAGS":     "",
		"CGO_ENABLED": "0",
		"LDFLAGS":     b.ldFlags,
	}

	bin := path.Join("bin", cmd)
	if len(goos) != 0 && len(goarch) != 0 {
		// change bin path to point to a sudirectory when cross compiling
		bin = path.Join("bin", goos+"_"+goarch, cmd)
		env["GOOS"] = goos
		env["GOARCH"] = goarch
	}

	if err := sh.RunWithV(
		env,
		"go", "build", "-v", "-o", bin, "./cmd/"+cmd+"/main.go",
	); err != nil {
		return fmt.Errorf("compiling cmd/%s: %w", cmd, err)
	}
	return nil
}

func (b *builder) Image(name string) error {
	switch name {
	case "package-operator", "coordination-operator", "nginx":
		return b.buildPackageImage(name)
	}
	return b.buildCmdImage(name)
}

// clean/prepare cache directory
func (b *builder) cleanImageCacheDir(name string) (dir string, err error) {
	imageCacheDir := path.Join(cacheDir, "image", name)
	if err := os.RemoveAll(imageCacheDir); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("deleting image cache: %w", err)
	}
	if err := os.Remove(imageCacheDir + ".tar"); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("deleting image cache: %w", err)
	}
	if err := os.MkdirAll(imageCacheDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("create image cache dir: %w", err)
	}
	return imageCacheDir, nil
}

// generic image build function, when the image just relies on
// a static binary build from cmd/*
func (b *builder) buildCmdImage(cmd string) error {
	mg.SerialDeps(
		b.init,
		determineContainerRuntime,
		mg.F(b.Cmd, cmd, "linux", "amd64"),
	)

	imageCacheDir, err := b.cleanImageCacheDir(cmd)
	if err != nil {
		return err
	}

	// prepare build context
	imageTag := b.imageURL(cmd)
	for _, command := range [][]string{
		// Copy files for build environment
		{"cp", "-a",
			path.Join("bin/linux_amd64", cmd),
			path.Join(imageCacheDir, cmd)},
		{"cp", "-a",
			path.Join("config/images", cmd+".Containerfile"),
			path.Join(imageCacheDir, "Containerfile")},
		{"cp", "-a",
			path.Join("config/images", "passwd"),
			path.Join(imageCacheDir, "passwd")},
	} {
		if err := sh.Run(command[0], command[1:]...); err != nil {
			return fmt.Errorf("running %q: %w", strings.Join(command, " "), err)
		}
	}

	for _, command := range [][]string{
		// Build image!
		{containerRuntime, "build", "-t", imageTag, "-f", "Containerfile", "."},
		{containerRuntime, "image", "save",
			"-o", imageCacheDir + ".tar", imageTag},
	} {
		buildCmd := exec.Command(command[0], command[1:]...)
		buildCmd.Stderr = os.Stderr
		buildCmd.Stdout = os.Stdout
		buildCmd.Dir = imageCacheDir
		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("running %q: %w", strings.Join(command, " "), err)
		}
	}

	return nil
}

func (b *builder) buildPackageImage(packageName string) error {
	mg.SerialDeps(
		b.init,
		determineContainerRuntime,
	)

	imageCacheDir, err := b.cleanImageCacheDir(packageName)
	if err != nil {
		return err
	}

	imageTag := b.imageURL(packageName)
	for _, command := range [][]string{
		// Copy files for build environment
		{"cp", "-a",
			path.Join("config/packages", packageName) + "/.",
			imageCacheDir + "/"},
		{"cp", "-a",
			"config/images/package.Containerfile",
			path.Join(imageCacheDir, "Containerfile")},

		// Build image!
		{containerRuntime, "build", "-t", imageTag, imageCacheDir},
		{containerRuntime, "image", "save",
			"-o", imageCacheDir + ".tar", imageTag},
	} {
		if err := sh.Run(command[0], command[1:]...); err != nil {
			return fmt.Errorf("running %q: %w", strings.Join(command, " "), err)
		}
	}
	return nil
}

func (b *builder) Push(imageName string) error {
	mg.SerialDeps(
		mg.F(b.Image, imageName),
	)

	// Login to container registry when running on AppSRE Jenkins.
	if _, ok := os.LookupEnv("JENKINS_HOME"); ok {
		log.Println("running in Jenkins, calling container runtime login")
		if err := sh.Run(containerRuntime,
			"login", "-u="+os.Getenv("QUAY_USER"),
			"-p="+os.Getenv("QUAY_TOKEN"), "quay.io"); err != nil {
			return fmt.Errorf("registry login: %w", err)
		}
	}

	if err := sh.Run(containerRuntime, "push", b.imageURL(imageName)); err != nil {
		return fmt.Errorf("pushing image: %w", err)
	}

	return nil
}

func (b *builder) imageURL(name string) string {
	envvar := strings.ReplaceAll(strings.ToUpper(name), "-", "_") + "_IMAGE"
	if url := os.Getenv(envvar); len(url) != 0 {
		return url
	}
	return b.imageOrg + "/" + name + ":" + b.version
}

// Development
// -----------

type Dev mg.Namespace

var (
	devEnvironment *dev.Environment
)

func (d Dev) Setup(ctx context.Context) error {
	mg.SerialDeps(
		Dev.init,
	)

	if err := devEnvironment.Init(ctx); err != nil {
		return fmt.Errorf("initializing dev environment: %w", err)
	}
	return nil
}

func (d Dev) Teardown(ctx context.Context) error {
	mg.SerialDeps(
		Dev.init,
	)

	if err := devEnvironment.Destroy(ctx); err != nil {
		return fmt.Errorf("tearing down dev environment: %w", err)
	}
	return nil
}

// Deploy the Package Operator.
// All components are deployed via static manifests.
func (d Dev) Deploy(ctx context.Context) error {
	mg.SerialDeps(
		Dev.Setup, // setup is a pre-requisite and needs to run before we can load images.
		mg.F(Dev.LoadImage, "package-operator-manager"),
	)

	if err := d.deployPackageOperatorManager(ctx, devEnvironment.Cluster); err != nil {
		return fmt.Errorf("deploying: %w", err)
	}
	return nil
}

// deploy the Package Operator Manager from local files.
func (d Dev) deployPackageOperatorManager(ctx context.Context, cluster *dev.Cluster) error {
	objs, err := dev.LoadKubernetesObjectsFromFile(
		"config/static-deployment/deployment.yaml.tpl")
	if err != nil {
		return fmt.Errorf("loading package-operator-manager deployment.yaml.tpl: %w", err)
	}

	// Replace image
	packageOperatorDeployment := &appsv1.Deployment{}
	if err := cluster.Scheme.Convert(
		&objs[0], packageOperatorDeployment, nil); err != nil {
		return fmt.Errorf("converting to Deployment: %w", err)
	}
	packageOperatorManagerImage := os.Getenv("PACKAGE_OPERATOR_MANAGER_IMAGE")
	if len(packageOperatorManagerImage) == 0 {
		packageOperatorManagerImage = Builder.imageURL("package-operator-manager")
	}
	for i := range packageOperatorDeployment.Spec.Template.Spec.Containers {
		container := &packageOperatorDeployment.Spec.Template.Spec.Containers[i]

		switch container.Name {
		case "manager":
			container.Image = packageOperatorManagerImage
		}
	}

	ctx = dev.ContextWithLogger(ctx, logger)

	// Deploy
	if err := cluster.CreateAndWaitFromFolders(ctx, []string{
		"config/static-deployment",
	}); err != nil {
		return fmt.Errorf("deploy package-operator-manager dependencies: %w", err)
	}
	if err := cluster.CreateAndWaitForReadiness(ctx, packageOperatorDeployment); err != nil {
		return fmt.Errorf("deploy package-operator-manager: %w", err)
	}
	return nil
}

// Setup local dev environment with the package operator installed and run the integration test suite.
func (d Dev) Integration(ctx context.Context) error {
	mg.SerialDeps(
		Dev.Deploy,
	)

	os.Setenv("KUBECONFIG", devEnvironment.Cluster.Kubeconfig())

	mg.SerialDeps(Test.Integration)
	return nil
}

func (d Dev) LoadImage(ctx context.Context, image string) error {
	mg.Deps(
		mg.F(Build.Image, image),
	)

	imageTar := path.Join(cacheDir, "image", image+".tar")
	if err := devEnvironment.LoadImageFromTar(ctx, imageTar); err != nil {
		return fmt.Errorf("load image from tar: %w", err)
	}
	return nil
}

func (d Dev) init() {
	mg.SerialDeps(
		determineContainerRuntime,
		Dependency.Kind,
	)

	devEnvironment = dev.NewEnvironment(
		"package-operator-dev",
		path.Join(cacheDir, "dev-env"),
		dev.WithClusterOptions([]dev.ClusterOption{
			dev.WithWaitOptions([]dev.WaitOption{
				dev.WithTimeout(2 * time.Minute),
			}),
		}),
		dev.WithContainerRuntime(containerRuntime),
	)
}
