//go:build mage
// +build mage

package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/mt-sre/devkube/dev"
	"github.com/mt-sre/devkube/magedeps"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientScheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	module                    = "package-operator.run/package-operator"
	defaultImageOrg           = "quay.io/package-operator"
	clusterName               = "package-operator-dev"
	pkoPackageName            = "package-operator-package"
	pkoManagerBinaryImageName = "package-operator-manager"
)

// Dependency Versions
const (
	controllerGenVersion = "0.6.2"
	goimportsVersion     = "0.1.5"
	golangciLintVersion  = "1.50.1"
	kindVersion          = "0.16.0"
	k8sDocGenVersion     = "0.5.1"
)

var (
	multiArchTargets = [][2]string{
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"windows", "amd64"},
		{"darwin", "amd64"},
		{"darwin", "arm64"},
	}
)

var (
	commandImagePath                       = filepath.Join("config", "images", "commands")
	packageImagePath                       = filepath.Join("config", "images", "packages")
	packageImageContainerFile              = filepath.Join("config", "images", "packages", "package.Containerfile")
	webhookPath                            = filepath.Join("config", "deploy", "webhook")
	staticDeploymentPath                   = filepath.Join("config", "static-deployment")
	remotePhaseManagerStaticDeploymentPath = filepath.Join("config", "remote-phase-static-deployment")
	containerFileSuffix                    = ".Containerfile"
)

var (
	workDir  string                       // Working directory of the project.
	depsDir  magedeps.DependencyDirectory // Dependency directory.
	cacheDir string

	containerRuntime string

	// components
	logger  logr.Logger
	Builder = &builder{}
)

func init() {
	var err error
	// Directories
	workDir, err = os.Getwd()
	if err != nil {
		panic(fmt.Errorf("getting work dir: %w", err))
	}
	cacheDir = filepath.Join(workDir, ".cache")
	depsDir = magedeps.DependencyDirectory(filepath.Join(workDir, ".deps"))
	os.Setenv("PATH", depsDir.Bin()+":"+os.Getenv("PATH"))

	// Use a local directory to get around permission errors in OpenShift CI.
	os.Setenv("GOLANGCI_LINT_CACHE", filepath.Join(cacheDir, "golangci-lint"))

	logger = stdr.New(nil)

	if err := Builder.init(); err != nil {
		panic(err)
	}
}

func allCommands() []string {
	cmdEntries, err := os.ReadDir("cmd")
	if err != nil {
		panic(fmt.Errorf("search for project commands: %w", err))
	}

	cmds := []string{}
	for _, entry := range cmdEntries {
		name := entry.Name()
		if entry.IsDir() && name != "mage" {
			cmds = append(cmds, name)
		}
	}

	return cmds
}

func allPackageImages() []string {
	entries, err := os.ReadDir(packageImagePath)
	if err != nil {
		panic(fmt.Errorf("finding package images: %w", err))
	}
	images := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			images = append(images, entry.Name())
		}
	}

	return images
}

func allCommandImages() []string {
	entries, err := os.ReadDir(commandImagePath)
	if err != nil {
		panic(fmt.Errorf("finding command images: %w", err))
	}
	images := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasSuffix(name, containerFileSuffix) {
			images = append(images, strings.TrimSuffix(name, containerFileSuffix))
		}
	}

	return images
}

func allImages() []string { return append(allPackageImages(), allCommandImages()...) }

// Testing and Linting
// -------------------
type Test mg.Namespace

// Runs linters.
func (Test) Lint() error {
	mg.Deps(
		Generate.All, // ensure code generators are re-triggered
		Dependency.GolangciLint,
	)

	cmds := [][]string{
		{"go", "fmt", "./..."},
		{"golangci-lint", "run", "./...", "--deadline=15m", "--fix"},
		{"bash", filepath.Join("hack", "validate-directory-clean.sh")},
	}

	for _, cmd := range cmds {
		if err := sh.RunV(cmd[0], cmd[1:]...); err != nil {
			return fmt.Errorf("running %q: %w", strings.Join(cmd, " "), err)
		}
	}

	return nil
}

// Runs unittests.
func (Test) Unit() error {
	codeCov := filepath.Join(cacheDir, "unit", "cov.out")
	execReport := filepath.Join(cacheDir, "unit", "exec.json")
	if err := os.MkdirAll(filepath.Dir(codeCov), os.ModePerm); err != nil {
		return err
	}

	_, isCI := os.LookupEnv("CI")
	testCmd := fmt.Sprintf("go test -coverprofile=%s -race", codeCov)
	if isCI {
		// test output in json format
		testCmd += " -json"
	}
	testCmd += " ./internal/... ./cmd/..."

	if isCI {
		testCmd = testCmd + " > " + execReport
	}

	// cgo needed to enable race detector -race
	return sh.RunWithV(map[string]string{"CGO_ENABLED": "1"}, "bash", "-c", testCmd)
}

// Runs PKO integration tests against whatever cluster your KUBECONFIG is pointing at.
func (t Test) Integration(ctx context.Context) error { return t.integration(ctx, "") }

// Runs PKO integration tests against whatever cluster your KUBECONFIG is pointing at.
// Also allows specifying only sub tests to run e.g. ./mage test:integrationrun TestPackage_success
func (t Test) IntegrationRun(ctx context.Context, filter string) error {
	return t.integration(ctx, filter)
}

func (Test) integration(ctx context.Context, filter string) error {
	os.Setenv("PKO_TEST_SUCCESS_PACKAGE_IMAGE", Builder.imageURL("test-stub-package"))
	os.Setenv("PKO_TEST_STUB_IMAGE", Builder.imageURL("test-stub"))

	// count=1 will force a new run, instead of using the cache
	args := []string{"test", "-v", "-failfast", "-count=1", "-timeout=20m"}
	if len(filter) > 0 {
		args = append(args, "-run", filter)
	}
	args = append(args, "./integration/...")
	testErr := sh.Run("go", args...)

	// always export logs
	if devEnvironment != nil {
		args := []string{"export", "logs", filepath.Join(cacheDir, "dev-env-logs"), "--name", clusterName}
		if err := devEnvironment.RunKindCommand(ctx, os.Stdout, os.Stderr, args...); err != nil {
			logger.Error(err, "exporting logs")
		}
	}

	return testErr
}

// Building
// --------
type Build mg.Namespace

// Build all PKO binaries for the architecture of this machine.
func (Build) Binaries() {
	targets := []interface{}{mg.F(Builder.Cmd, "mage", "", "")}
	for _, cmd := range allCommands() {
		targets = append(targets, mg.F(Builder.Cmd, cmd, runtime.GOOS, runtime.GOARCH))
	}

	mg.Deps(targets...)
}

func (Build) MultiArchBinaries() {
	targets := []interface{}{mg.F(Builder.Cmd, "mage", "", "")}
	for _, cmd := range allCommands() {
		for _, archTarget := range multiArchTargets {
			targets = append(targets, mg.F(Builder.Cmd, cmd, archTarget[0], archTarget[1]))
		}
	}

	mg.Deps(targets...)
}

func (Build) Binary(cmd string) { mg.Deps(mg.F(Builder.Cmd, cmd, runtime.GOOS, runtime.GOARCH)) }

// Builds the given container image, building binaries as prerequisite as required.
func (Build) Image(image string) { mg.Deps(mg.F(Builder.Image, image)) }

// Builds all PKO container images.
func (Build) Images() {
	mg.Deps(
		mg.F(Builder.Image, pkoManagerBinaryImageName),
		mg.F(Builder.Image, "package-operator-webhook"),
		mg.F(Builder.Image, pkoPackageName),
		mg.F(Builder.Image, "remote-phase-manager"),
	)
}

// Builds and pushes only the given container image to the default registry.
func (Build) PushImage(image string) { mg.Deps(mg.F(Builder.Push, image)) }

// Builds and pushes all container images to the default registry.
func (Build) PushImages() {
	mg.Deps(
		mg.F(Builder.Push, pkoManagerBinaryImageName),
		mg.F(Builder.Push, "package-operator-webhook"),
		mg.F(Builder.Push, pkoPackageName),
		mg.F(Builder.Push, "remote-phase-manager"),
	)
	mg.SerialDeps(Generate.SelfBootstrapJob)
}

// Dependencies
// ------------

type Dependency mg.Namespace

// Installs all project dependencies into the local checkout.
func (d Dependency) All() {
	mg.Deps(Dependency.ControllerGen, Dependency.Goimports, Dependency.GolangciLint, Dependency.Kind, Dependency.Docgen)
}

// Ensure controller-gen - kubebuilder code and manifest generator.
func (d Dependency) ControllerGen() error {
	url := "sigs.k8s.io/controller-tools/cmd/controller-gen"
	return depsDir.GoInstall("controller-gen", url, controllerGenVersion)
}

func (d Dependency) Goimports() error {
	url := "golang.org/x/tools/cmd/goimports"
	return depsDir.GoInstall("go-imports", url, goimportsVersion)
}

func (d Dependency) GolangciLint() error {
	url := "github.com/golangci/golangci-lint/cmd/golangci-lint"
	return depsDir.GoInstall("golangci-lint", url, golangciLintVersion)
}

func (d Dependency) Docgen() error {
	url := "github.com/thetechnick/k8s-docgen"
	return depsDir.GoInstall("k8s-docgen", url, k8sDocGenVersion)
}

// Ensure Kind dependency - Kubernetes in Docker (or Podman)
func (d Dependency) Kind() error {
	url := "sigs.k8s.io/kind"
	return depsDir.GoInstall("kind", url, kindVersion)
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
	// Build Tags
	imageOrg string
	version  string
}

// init build variables
func (b *builder) init() error {
	// Use version from VERSION env if present, use "git describe" elsewise.
	b.version = strings.TrimSpace(os.Getenv("VERSION"))
	if len(b.version) == 0 {
		gitDescribeCmd := exec.Command("git", "describe", "--tags")
		version, err := gitDescribeCmd.Output()
		if err != nil {
			panic(fmt.Errorf("git describe: %w", err))
		}
		b.version = strings.TrimSpace(string(version))
	}

	// image org
	b.imageOrg = os.Getenv("IMAGE_ORG")
	if len(b.imageOrg) == 0 {
		b.imageOrg = defaultImageOrg
	}

	return nil
}

// Builds binaries from /cmd directory.
func (b *builder) Cmd(cmd, goos, goarch string) error {
	mg.SerialDeps(b.init)

	env := map[string]string{"CGO_ENABLED": "0"}

	bin := filepath.Join("bin", cmd)
	if len(goos) != 0 && len(goarch) != 0 {
		// change bin path to point to a sudirectory when cross compiling
		bin = filepath.Join("bin", goos+"_"+goarch, cmd)
		env["GOOS"] = goos
		env["GOARCH"] = goarch
	}

	ldflags := "-w -s --extldflags '-zrelro -znow -O1'" + fmt.Sprintf("-X '%s/internal/version.version=%s'", module, b.version)
	cmdline := []string{"build", "--ldflags", ldflags, "--trimpath", "--mod=readonly", "-v", "-o", bin, "./cmd/" + cmd}

	if err := sh.RunWithV(env, "go", cmdline...); err != nil {
		return fmt.Errorf("compiling cmd/%s: %w", cmd, err)
	}

	return nil
}

func (b *builder) Image(name string) error {
	if strings.HasSuffix(name, "-package") {
		return b.buildPackageImage(name)
	}

	return b.buildCmdImage(name)
}

// clean/prepare cache directory
func (b *builder) cleanImageCacheDir(name string) (dir string, err error) {
	imageCacheDir := filepath.Join(cacheDir, "image", name)
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
	mg.SerialDeps(b.init, determineContainerRuntime, mg.F(b.Cmd, cmd, "linux", "amd64"))

	imageCacheDir, err := b.cleanImageCacheDir(cmd)
	if err != nil {
		return err
	}

	// prepare build context
	imageTag := b.imageURL(cmd)
	// Copy files for build environment
	cmds := [][]string{
		{"cp", "-a", filepath.Join("bin/linux_amd64", cmd), filepath.Join(imageCacheDir, cmd)},
		{"cp", "-a", filepath.Join("config/images", cmd+".Containerfile"), filepath.Join(imageCacheDir, "Containerfile")},
		{"cp", "-a", filepath.Join("config/images", "passwd"), filepath.Join(imageCacheDir, "passwd")},
	}
	for _, command := range cmds {
		if err := sh.Run(command[0], command[1:]...); err != nil {
			return fmt.Errorf("running %q: %w", strings.Join(command, " "), err)
		}
	}

	// Build image!
	cmds = [][]string{
		{containerRuntime, "build", "-t", imageTag, "-f", "Containerfile", "."},
		{containerRuntime, "image", "save", "-o", imageCacheDir + ".tar", imageTag},
	}

	for _, command := range cmds {
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

func (b *builder) buildPackageImage(packageImageName string) error {
	mg.SerialDeps(b.init, determineContainerRuntime)
	if packageImageName == pkoPackageName {
		// inject digests into package
		mg.SerialDeps(Generate.PackageOperatorPackage)
	}

	imageCacheDir, err := b.cleanImageCacheDir(packageImageName)
	if err != nil {
		return err
	}

	imageTag := b.imageURL(packageImageName)
	packageName := strings.TrimSuffix(packageImageName, "-package")

	// Copy files for build environment
	cmds := [][]string{
		{"cp", "-a", filepath.Join("config/packages", packageName) + "/.", imageCacheDir + "/"},
		{"cp", "-a", "config/images/package.Containerfile", filepath.Join(imageCacheDir, "Containerfile")},
	}

	for _, command := range cmds {
		if err := sh.Run(command[0], command[1:]...); err != nil {
			return fmt.Errorf("running %q: %w", strings.Join(command, " "), err)
		}
	}

	// Build image!
	cmds = [][]string{
		{containerRuntime, "build", "-t", imageTag, "-f", "Containerfile", "."},
		{containerRuntime, "image", "save", "-o", imageCacheDir + ".tar", imageTag},
	}

	for _, command := range cmds {
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

func (b *builder) Push(imageName string) error {
	mg.SerialDeps(mg.F(b.Image, imageName))

	// Login to container registry when running on AppSRE Jenkins.
	_, isJenkins := os.LookupEnv("JENKINS_HOME")
	_, isCI := os.LookupEnv("CI")
	if isJenkins || isCI {
		log.Println("running in CI, calling container runtime login")
		args := []string{"login", "-u=" + os.Getenv("QUAY_USER"), "-p=" + os.Getenv("QUAY_TOKEN"), "quay.io"}
		if err := sh.Run(containerRuntime, args...); err != nil {
			return fmt.Errorf("registry login: %w", err)
		}
	}

	args := []string{"push"}
	if containerRuntime == string(dev.ContainerRuntimePodman) {
		args = append(args, "--digestfile="+digestFile(imageName))
	}
	args = append(args, b.imageURL(imageName))

	if err := sh.Run(containerRuntime, args...); err != nil {
		return fmt.Errorf("pushing image: %w", err)
	}

	return nil
}

func (b *builder) imageURL(name string) string {
	return b.internalImageURL(name, false)
}

func (b *builder) imageURLWithDigest(name string) string {
	return b.internalImageURL(name, true)
}

func digestFile(imageName string) string {
	return filepath.Join(cacheDir, imageName+".digest")
}

func (b *builder) internalImageURL(name string, useDigest bool) string {
	envvar := strings.ReplaceAll(strings.ToUpper(name), "-", "_") + "_IMAGE"
	if url := os.Getenv(envvar); len(url) != 0 {
		return url
	}
	image := b.imageOrg + "/" + name + ":" + b.version
	if !useDigest {
		return image
	}

	digest, err := os.ReadFile(digestFile(name))
	if err != nil {
		panic(err)
	}

	return b.imageOrg + "/" + name + "@" + string(digest)
}

// Development
// -----------

type Dev mg.Namespace

var (
	devEnvironment *dev.Environment
)

// Creates an empty development environment via kind.
func (d Dev) Setup(ctx context.Context) error {
	mg.SerialDeps(Dev.init)

	if err := devEnvironment.Init(ctx); err != nil {
		return fmt.Errorf("initializing dev environment: %w", err)
	}
	return nil
}

// Tears the whole kind development environment down.
func (d Dev) Teardown(ctx context.Context) error {
	mg.SerialDeps(Dev.init)

	if err := devEnvironment.Destroy(ctx); err != nil {
		return fmt.Errorf("tearing down dev environment: %w", err)
	}
	return nil
}

// Load images into the development environment.
func (d Dev) Load() {
	// setup is a pre-requisite and needs to run before we can load images.
	mg.SerialDeps(Dev.Setup)
	images := []string{
		pkoPackageName, pkoManagerBinaryImageName, "package-operator-webhook",
		"remote-phase-manager", "test-stub", "test-stub-package",
	}
	deps := make([]interface{}, len(images))
	for i := range images {
		deps[i] = mg.F(Dev.LoadImage, images[i])
	}
	mg.Deps(deps...)

	mg.SerialDeps(Generate.SelfBootstrapJob)

	// Print all Loaded images, so we can reference them manually.
	fmt.Println("----------------------------")
	fmt.Println("loaded images into kind cluster:")
	for i := range images {
		fmt.Println(Builder.imageURL(images[i]))
	}
	fmt.Println("----------------------------")
}

// Setup local cluster and deploy the Package Operator.
func (d Dev) Deploy(ctx context.Context) error {
	mg.SerialDeps(Dev.Load)

	if err := d.deployPackageOperatorManager(ctx, devEnvironment.Cluster); err != nil {
		return fmt.Errorf("deploying: %w", err)
	}
	if err := d.deployPackageOperatorWebhook(ctx, devEnvironment.Cluster); err != nil {
		return fmt.Errorf("deploying: %w", err)
	}
	if err := d.deployRemotePhaseManager(ctx, devEnvironment.Cluster); err != nil {
		return fmt.Errorf("deploying: %w", err)
	}
	return nil
}

// deploy the Package Operator Manager from local files.
func (d Dev) deployPackageOperatorManager(ctx context.Context, cluster *dev.Cluster) error {
	packageOperatorDeployment, err := templatePackageOperatorManager(cluster.Scheme)
	if err != nil {
		return err
	}

	ctx = logr.NewContext(ctx, logger)

	// Deploy
	if err := cluster.CreateAndWaitFromFolders(ctx, []string{staticDeploymentPath}); err != nil {
		return fmt.Errorf("deploy package-operator-manager dependencies: %w", err)
	}
	_ = cluster.CtrlClient.Delete(ctx, packageOperatorDeployment)
	if err := cluster.CreateAndWaitForReadiness(ctx, packageOperatorDeployment); err != nil {
		return fmt.Errorf("deploy package-operator-manager: %w", err)
	}
	return nil
}

func templatePackageOperatorManager(scheme *k8sruntime.Scheme) (deploy *appsv1.Deployment, err error) {
	objs, err := dev.LoadKubernetesObjectsFromFile(filepath.Join(staticDeploymentPath, "deployment.yaml.tpl"))
	if err != nil {
		return nil, fmt.Errorf("loading package-operator-manager deployment.yaml.tpl: %w", err)
	}

	return patchPackageOperatorManager(scheme, &objs[0])
}

func patchPackageOperatorManager(scheme *k8sruntime.Scheme, obj *unstructured.Unstructured) (deploy *appsv1.Deployment, err error) {
	// Replace image
	packageOperatorDeployment := &appsv1.Deployment{}
	if err := scheme.Convert(
		obj, packageOperatorDeployment, nil); err != nil {
		return nil, fmt.Errorf("converting to Deployment: %w", err)
	}

	var packageOperatorManagerImage string
	if len(os.Getenv("USE_DIGESTS")) > 0 {
		// to use digests the image needs to be pushed to a registry first.
		mg.Deps(mg.F(Builder.Push, pkoManagerBinaryImageName))
		packageOperatorManagerImage = Builder.imageURLWithDigest(pkoManagerBinaryImageName)
	} else {
		packageOperatorManagerImage = Builder.imageURL(pkoManagerBinaryImageName)
	}

	for i := range packageOperatorDeployment.Spec.Template.Spec.Containers {
		container := &packageOperatorDeployment.Spec.Template.Spec.Containers[i]

		switch container.Name {
		case "manager":
			container.Image = packageOperatorManagerImage

			for j := range container.Env {
				env := &container.Env[j]
				if env.Name == "PKO_IMAGE" {
					env.Value = packageOperatorManagerImage
				}
			}
		}
	}
	return packageOperatorDeployment, nil
}

// Package Operator Webhook server from local files.
func (d Dev) deployPackageOperatorWebhook(ctx context.Context, cluster *dev.Cluster) error {
	objs, err := dev.LoadKubernetesObjectsFromFile("config/deploy/webhook/deployment.yaml.tpl")
	if err != nil {
		return fmt.Errorf("loading package-operator-webhook deployment.yaml.tpl: %w", err)
	}

	// Replace image
	packageOperatorWebhookDeployment := &appsv1.Deployment{}
	if err := cluster.Scheme.Convert(&objs[0], packageOperatorWebhookDeployment, nil); err != nil {
		return fmt.Errorf("converting to Deployment: %w", err)
	}
	packageOperatorWebhookImage := os.Getenv("PACKAGE_OPERATOR_WEBHOOK_IMAGE")
	if len(packageOperatorWebhookImage) == 0 {
		packageOperatorWebhookImage = Builder.imageURL("package-operator-webhook")
	}
	for i := range packageOperatorWebhookDeployment.Spec.Template.Spec.Containers {
		container := &packageOperatorWebhookDeployment.Spec.Template.Spec.Containers[i]

		switch container.Name {
		case "webhook":
			container.Image = packageOperatorWebhookImage
		}
	}

	ctx = logr.NewContext(ctx, logger)

	// Deploy
	if err := cluster.CreateAndWaitFromFiles(ctx, []string{
		"config/deploy/webhook/00-tls-secret.yaml",
		"config/deploy/webhook/service.yaml.tpl",
		"config/deploy/webhook/objectsetvalidatingwebhookconfig.yaml",
		"config/deploy/webhook/objectsetphasevalidatingwebhookconfig.yaml",
		"config/deploy/webhook/clusterobjectsetvalidatingwebhookconfig.yaml",
		"config/deploy/webhook/clusterobjectsetphasevalidatingwebhookconfig.yaml",
	}); err != nil {
		return fmt.Errorf("deploy package-operator-webhook dependencies: %w", err)
	}
	_ = cluster.CtrlClient.Delete(ctx, packageOperatorWebhookDeployment)
	if err := cluster.CreateAndWaitForReadiness(ctx, packageOperatorWebhookDeployment); err != nil {
		return fmt.Errorf("deploy package-operator-webhook: %w", err)
	}
	return nil
}

// Remote phase manager from local files.
func (d Dev) deployRemotePhaseManager(ctx context.Context, cluster *dev.Cluster) error {
	objs, err := dev.LoadKubernetesObjectsFromFile(filepath.Join(remotePhaseManagerStaticDeploymentPath, "deployment.yaml.tpl"))
	if err != nil {
		return fmt.Errorf("loading package-operator-webhook deployment.yaml.tpl: %w", err)
	}

	// Insert new image in remote-phase-manager deployment manifest
	remotePhaseManagerDeployment := &appsv1.Deployment{}
	if err := cluster.Scheme.Convert(&objs[0], remotePhaseManagerDeployment, nil); err != nil {
		return fmt.Errorf("converting to Deployment: %w", err)
	}
	packageOperatorWebhookImage := os.Getenv("REMOTE_PHASE_MANAGER_IMAGE")
	if len(packageOperatorWebhookImage) == 0 {
		packageOperatorWebhookImage = Builder.imageURL("remote-phase-manager")
	}
	for i := range remotePhaseManagerDeployment.Spec.Template.Spec.Containers {
		container := &remotePhaseManagerDeployment.Spec.Template.Spec.Containers[i]

		switch container.Name {
		case "manager":
			container.Image = packageOperatorWebhookImage
		}
	}

	// Beware: CreateAndWaitFromFolders doesn't update anything
	// Create the service accounts and related dependencies
	if err := cluster.CreateAndWaitFromFolders(ctx, []string{remotePhaseManagerStaticDeploymentPath}); err != nil {
		return fmt.Errorf("deploy remote-phase-manager dependencies: %w", err)
	}

	// Get Kubeconfig, will be edited for the target service account
	targetKubeconfigPath := os.Getenv("TARGET_KUBECONFIG_PATH")
	var kubeconfigBytes []byte
	if len(targetKubeconfigPath) == 0 {
		kubeconfigBuf := new(bytes.Buffer)
		args := []string{"get", "kubeconfig", "--name", clusterName, "--internal"}
		err = devEnvironment.RunKindCommand(ctx, kubeconfigBuf, os.Stderr, args...)
		if err != nil {
			return fmt.Errorf("exporting internal kubeconfig: %w", err)
		}
		kubeconfigBytes = kubeconfigBuf.Bytes()
		old := []byte("package-operator-dev-control-plane:6443")
		new := []byte("kubernetes.default")
		kubeconfigBytes = bytes.Replace(kubeconfigBytes, old, new, -1) // use in-cluster DNS
	} else {
		kubeconfigBytes, err = ioutil.ReadFile(targetKubeconfigPath)
		if err != nil {
			return fmt.Errorf("reading in kubeconfig: %w", err)
		}
	}

	kubeconfigMap := map[string]interface{}{}
	err = yaml.UnmarshalStrict(kubeconfigBytes, &kubeconfigMap)
	if err != nil {
		return fmt.Errorf("unmarshalling kubeconfig: %w", err)
	}

	// Get target cluster service account
	targetSASecret := &corev1.Secret{}
	key := client.ObjectKey{Namespace: "package-operator-system", Name: "remote-phase-operator-target-cluster"}
	err = cluster.CtrlClient.Get(context.TODO(), key, targetSASecret)
	if err != nil {
		return fmt.Errorf("reading in service account secret: %w", err)
	}

	// Insert target cluster service account token into kubeconfig
	kubeconfigMap["users"] = []map[string]interface{}{
		{
			"name": "kind-package-operator-dev",
			"user": map[string]string{
				"token": string(targetSASecret.Data["token"]),
			},
		},
	}

	newKubeconfigBytes, err := yaml.Marshal(kubeconfigMap)
	if err != nil {
		return fmt.Errorf("marshalling new kubeconfig back to yaml: %w", err)
	}

	// Create a new secret for the kubeconfig
	secret := &corev1.Secret{}
	objs, err = dev.LoadKubernetesObjectsFromFile(filepath.Join(remotePhaseManagerStaticDeploymentPath, "2-secret.yaml.tpl"))
	if err != nil {
		return fmt.Errorf("loading package-operator-webhook 2-secret.yaml.tpl: %w", err)
	}
	if err := cluster.Scheme.Convert(
		&objs[0], secret, nil); err != nil {
		return fmt.Errorf("converting to Secret: %w", err)
	}

	// insert the new kubeconfig into the secret
	secret.Data = map[string][]byte{"kubeconfig": newKubeconfigBytes}

	ctx = logr.NewContext(ctx, logger)

	// Deploy the secret with the new kubeconfig
	_ = cluster.CtrlClient.Delete(ctx, secret)
	if err := cluster.CreateAndWaitForReadiness(ctx, secret); err != nil {
		return fmt.Errorf("deploy kubeconfig secret: %w", err)
	}
	// Deploy the remote phase manager deployment
	_ = cluster.CtrlClient.Delete(ctx, remotePhaseManagerDeployment)
	if err := cluster.CreateAndWaitForReadiness(ctx, remotePhaseManagerDeployment); err != nil {
		return fmt.Errorf("deploy remote-phase-manager: %w", err)
	}
	return nil
}

// Setup local dev environment with the package operator installed and run the integration test suite.
func (d Dev) Integration(ctx context.Context) error {
	mg.SerialDeps(Dev.Deploy)

	os.Setenv("KUBECONFIG", devEnvironment.Cluster.Kubeconfig())

	mg.SerialDeps(Test.Integration)
	return nil
}

func (d Dev) LoadImage(ctx context.Context, image string) error {
	mg.Deps(mg.F(Build.Image, image))

	imageTar := filepath.Join(cacheDir, "image", image+".tar")
	if err := devEnvironment.LoadImageFromTar(ctx, imageTar); err != nil {
		return fmt.Errorf("load image from tar: %w", err)
	}
	return nil
}

func (d Dev) init() {
	mg.SerialDeps(determineContainerRuntime, Dependency.Kind)

	devEnvironment = dev.NewEnvironment(
		clusterName,
		filepath.Join(cacheDir, "dev-env"),
		dev.WithClusterOptions([]dev.ClusterOption{
			dev.WithWaitOptions([]dev.WaitOption{dev.WithTimeout(2 * time.Minute)}),
		}),
		dev.WithContainerRuntime(containerRuntime),
	)
}

// Code Generators
// ---------------
type Generate mg.Namespace

// Run all code generators.
func (Generate) All() {
	// installYamlFile has to come after code generation
	mg.SerialDeps(Generate.code, Generate.docs, Generate.installYamlFile)
}

func (Generate) code() error {
	mg.Deps(Dependency.ControllerGen)
	apiPath := filepath.Join(workDir, "apis")

	args := []string{
		"crd:crdVersions=v1,generateEmbeddedObjectMeta=true",
		"paths=./core/...",
		"output:crd:artifacts:config=../config/crds",
	}

	manifestsCmd := exec.Command("controller-gen", args...)
	manifestsCmd.Dir = apiPath
	manifestsCmd.Stdout = os.Stdout
	manifestsCmd.Stderr = os.Stderr
	if err := manifestsCmd.Run(); err != nil {
		return fmt.Errorf("generating kubernetes manifests: %w", err)
	}

	// code gen
	codeCmd := exec.Command("controller-gen", "object", "paths=./...")
	codeCmd.Dir = apiPath
	if err := codeCmd.Run(); err != nil {
		return fmt.Errorf("generating deep copy methods: %w", err)
	}

	crds, err := filepath.Glob(filepath.Join("config", "crds", "*.yaml"))
	if err != nil {
		return fmt.Errorf("finding CRDs: %w", err)
	}

	for _, crd := range crds {
		cmd := []string{"cp", crd, filepath.Join(staticDeploymentPath, "1-"+filepath.Base(crd))}
		if err := sh.RunV(cmd[0], cmd[1:]...); err != nil {
			return fmt.Errorf("running %q: %w", strings.Join(cmd, " "), err)
		}
	}

	return nil
}

func (Generate) docs() error {
	mg.Deps(Dependency.Docgen)

	return sh.Run(filepath.Join("hack", "docgen.sh"))
}

func (Generate) installYamlFile() error {
	return dumpManifestsFromFolder(staticDeploymentPath, "install.yaml")
}

// dumpManifestsFromFolder dumps all kubernets manifests from all files
// in the given folder into the output file. It does not recurse into subfolders.
// It dumps the manifests in lexical order based on file name.
func dumpManifestsFromFolder(folderPath string, outputPath string) error {
	folder, err := os.Open(folderPath)
	if err != nil {
		return fmt.Errorf("open %q: %w", folderPath, err)
	}
	defer folder.Close()

	files, err := folder.Readdir(-1)
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}
	sort.Sort(fileInfosByName(files))

	if _, err = os.Stat(outputPath); err == nil {
		err = os.Remove(outputPath)
		if err != nil {
			return fmt.Errorf("removing old file: %s", err)
		}
	}

	outputFile, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed opening file: %s", err)
	}
	defer outputFile.Close()
	for i, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(folderPath, file.Name())
		fileYaml, err := ioutil.ReadFile(filePath)
		cleanFileYaml := bytes.Trim(fileYaml, "-\n")
		if err != nil {
			return fmt.Errorf("reading %s: %w", filePath, err)
		}

		_, err = outputFile.Write(cleanFileYaml)
		if err != nil {
			return fmt.Errorf("failed appending manifest from file %s to output file: %s", file, err)
		}
		if i != len(files)-1 {
			_, err = outputFile.WriteString("\n---\n")
			if err != nil {
				return fmt.Errorf("failed appending --- %s to output file: %s", file, err)
			}
		} else {
			_, err = outputFile.WriteString("\n")
			if err != nil {
				return fmt.Errorf("failed appending new line %s to output file: %s", file, err)
			}
		}
	}
	return nil
}

// Sorts fs.FileInfo objects by basename.
type fileInfosByName []fs.FileInfo

func (x fileInfosByName) Len() int { return len(x) }

func (x fileInfosByName) Less(i, j int) bool {
	iName := filepath.Base(x[i].Name())
	jName := filepath.Base(x[j].Name())
	return iName < jName
}

func (x fileInfosByName) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

// Includes all static-deployment files in the package-operator-package.
func (Generate) PackageOperatorPackage() error {
	return filepath.WalkDir(staticDeploymentPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		return includeInPackageOperatorPackage(path)
	})
}

// generates a self-bootstrap-job.yaml based on the current VERSION.
// requires the images to have been build beforehand.
func (Generate) SelfBootstrapJob() error {
	mg.Deps(determineContainerRuntime)

	const (
		pkoDefaultManagerImage = "quay.io/package-operator/package-operator-manager:latest"
		pkoDefaultPackageImage = "quay.io/package-operator/package-operator-package:latest"
	)

	latestJob, err := os.ReadFile("config/self-bootstrap-job.yaml.tpl")
	if err != nil {
		return err
	}

	var (
		packageOperatorManagerImage string
		packageOperatorPackageImage string
	)
	if len(os.Getenv("USE_DIGESTS")) > 0 {
		mg.Deps(mg.F(Builder.Push, pkoManagerBinaryImageName), mg.F(Builder.Push, pkoPackageName))
		packageOperatorManagerImage = Builder.imageURLWithDigest(pkoManagerBinaryImageName)
		packageOperatorPackageImage = Builder.imageURLWithDigest(pkoPackageName)
	} else {
		packageOperatorManagerImage = Builder.imageURL(pkoManagerBinaryImageName)
		packageOperatorPackageImage = Builder.imageURL(pkoPackageName)
	}

	latestJob = bytes.ReplaceAll(latestJob, []byte(pkoDefaultManagerImage), []byte(packageOperatorManagerImage))
	latestJob = bytes.ReplaceAll(latestJob, []byte(pkoDefaultPackageImage), []byte(packageOperatorPackageImage))

	if err := os.WriteFile("config/self-bootstrap-job.yaml", latestJob, os.ModePerm); err != nil {
		return err
	}
	return nil
}

func includeInPackageOperatorPackage(file string) error {
	fileContent, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	objs, err := dev.LoadKubernetesObjectsFromBytes(fileContent)
	if err != nil {
		return err
	}
	for _, obj := range objs {
		if len(obj.Object) == 0 {
			continue
		}

		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		gk := obj.GroupVersionKind().GroupKind()

		var (
			subfolder    string
			objToMarshal interface{}
		)
		switch gk {
		case schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}:
			annotations["package-operator.run/phase"] = "crds"
			subfolder = "crds"
			objToMarshal = obj.Object

		case schema.GroupKind{Group: "", Kind: "Namespace"}:
			annotations["package-operator.run/phase"] = "namespace"
			objToMarshal = obj.Object

		case schema.GroupKind{Group: "", Kind: "ServiceAccount"},
			schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "Role"},
			schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "RoleBinding"},
			schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRoleBinding"}:
			annotations["package-operator.run/phase"] = "rbac"
			subfolder = "rbac"
			objToMarshal = obj.Object

		case schema.GroupKind{Group: "apps", Kind: "Deployment"}:
			annotations["package-operator.run/phase"] = "deploy"
			obj.SetAnnotations(annotations)
			deploy, err := patchPackageOperatorManager(clientScheme.Scheme, &obj)
			if err != nil {
				return err
			}
			deploy.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
			objToMarshal = deploy
		}
		obj.SetAnnotations(annotations)

		outFilePath := filepath.Join("config", "packages", "package-operator")
		if len(subfolder) > 0 {
			outFilePath = filepath.Join(outFilePath, subfolder)
		}

		if err := os.MkdirAll(outFilePath, os.ModePerm); err != nil {
			return fmt.Errorf("creating output directory")
		}
		outFilePath = filepath.Join(outFilePath, fmt.Sprintf("%s.%s.yaml", obj.GetName(), gk.Kind))

		outFile, err := os.Create(outFilePath)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer outFile.Close()

		yamlBytes, err := yaml.Marshal(objToMarshal)
		if err != nil {
			return err
		}

		packageNamespaceOverride := os.Getenv("PKO_PACKAGE_NAMESPACE_OVERRIDE")
		if len(packageNamespaceOverride) > 0 {
			logger.Info("replacing default package-operator-system namespace", "new namespace", packageNamespaceOverride)
			yamlBytes = bytes.ReplaceAll(yamlBytes, []byte("package-operator-system"), []byte(packageNamespaceOverride))
		}

		if _, err := outFile.Write(yamlBytes); err != nil {
			return err
		}
	}

	return nil
}

func Deploy(ctx context.Context) error {
	workDir := filepath.Join(cacheDir, "deploy")
	cluster, err := dev.NewCluster(workDir, dev.WithKubeconfigPath(os.Getenv("KUBECONFIG")))
	if err != nil {
		return nil
	}

	var d Dev
	if err := d.deployPackageOperatorManager(ctx, cluster); err != nil {
		return fmt.Errorf("deploying: %w", err)
	}
	if err := d.deployPackageOperatorWebhook(ctx, cluster); err != nil {
		return fmt.Errorf("deploying: %w", err)
	}
	return nil
}
