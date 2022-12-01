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
	projectName     = "package-operator"
	module          = "package-operator.run/package-operator"
	defaultImageOrg = "quay.io/package-operator"
	clusterName     = "package-operator-dev"

	controllerGenVersion = "0.6.2"
	goimportsVersion     = "0.1.5"
	golangciLintVersion  = "1.50.1"
	kindVersion          = "0.16.0"
	k8sDocGenVersion     = "0.5.1"

	cliCmdName = "kubectl-package"
)

var (
	commands = map[string]command{
		"package-operator-manager": {nil},
		"remote-phase-manager":     {nil},
		cliCmdName:                 {[]archTarget{linuxAMD64Arch, {"darwin", "amd64"}, {"darwin", "arm64"}}},
	}

	packageImages = map[string]PackageImage{
		"package-operator-package": {},
	}
	commandImages = map[string]CommandImage{
		"package-operator-manager": {},
		"package-operator-webhook": {},
		"remote-phase-manager":     {},
		"kubectl-package":          {},
	}
)

type PackageImage struct{}

type CommandImage struct{}

type archTarget struct{ OS, Arch string }

type command struct{ ReleaseArchitectures []archTarget }

type Paths struct {
	// Dependency directory.
	dependencies magedeps.DependencyDirectory
	cache        string
}

func findCache() string {
	xdgCache, xdgCacheOK := os.LookupEnv("XDG_CACHE_HOME")

	if xdgCacheOK {
		path := filepath.Join(xdgCache, projectName)
		if err := os.MkdirAll(path, 0o755); err == nil {
			return path
		} else {
			fmt.Println(err)
		}
	}

	home, homeOK := os.LookupEnv("HOME")

	if homeOK {
		path := filepath.Join(home, ".cache", projectName)
		if err := os.MkdirAll(path, 0o755); err == nil {
			return path
		} else {
			fmt.Println(err)
		}
	}

	// Use has no home. Cache on the floor.
	path := ".cache"
	if err := os.MkdirAll(path, 0o755); err == nil {
		return path
	} else {
		fmt.Println(err)
	}

	panic("no working cache directory found")
}

func (p Paths) DependencyBin() string            { return filepath.Join(p.dependencies.Bin()) }
func (p Paths) APISubmodule() string             { return "apis" }
func (p Paths) ClusterDeploymentCache() string   { return filepath.Join(p.cache, "deploy") }
func (p Paths) unitTestCache() string            { return filepath.Join(p.cache, "unit") }
func (p Paths) UnitTestCoverageReport() string   { return filepath.Join(p.unitTestCache(), "cov.out") }
func (p Paths) UnitTestExecReport() string       { return filepath.Join(p.unitTestCache(), "exec.json") }
func (p Paths) IntegrationTestLogs() string      { return filepath.Join(p.cache, "dev-env-logs") }
func (p Paths) ImageCache() string               { return filepath.Join(p.cache, "image") }
func (p Paths) DevCache() string                 { return filepath.Join(p.cache, "dev-env") }
func (p Paths) DigestFile(imgName string) string { return filepath.Join(p.cache, imgName+".digest") }
func (p Paths) APIReference() string             { return filepath.Join("docs", "api-reference.md") }
func (p Paths) binaryDst(name string, arch archTarget) string {
	if arch == nativeArch {
		return filepath.Join("bin", name)
	}

	if len(arch.OS) == 0 || len(arch.Arch) == 0 {
		panic("invalid os or arch")
	}

	return filepath.Join("bin", arch.OS+"_"+arch.Arch, name)
}

func newPathSelector() Paths {
	p := Paths{}
	p.cache = findCache()

	p.dependencies = magedeps.DependencyDirectory(filepath.Join(p.cache, "dependencies"))
	must(os.MkdirAll(string(p.dependencies), 0o755))
	must(os.MkdirAll(p.unitTestCache(), 0o755))

	return p
}

var (
	nativeArch     = archTarget{runtime.GOOS, runtime.GOARCH}
	linuxAMD64Arch = archTarget{"linux", "amd64"}

	devEnvironment *dev.Environment

	paths = newPathSelector()

	logger           logr.Logger
	containerRuntime string

	// components
	Builder = &builder{}
)

func init() {
	os.Setenv("PATH", paths.DependencyBin()+":"+os.Getenv("PATH"))

	// Use a local directory to get around permission errors in OpenShift CI.
	os.Setenv("GOLANGCI_LINT_CACHE", filepath.Join(paths.cache, "golangci-lint"))

	logger = stdr.New(nil)

	Builder.init()
}

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

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func includeInPackageOperatorPackage(file string) {
	fileContent, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	objs, err := dev.LoadKubernetesObjectsFromBytes(fileContent)
	if err != nil {
		panic(err)
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
			deploy := patchPackageOperatorManager(clientScheme.Scheme, &obj)
			deploy.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
			objToMarshal = deploy
		}
		obj.SetAnnotations(annotations)

		outFilePath := filepath.Join("config", "packages", "package-operator")
		if len(subfolder) > 0 {
			outFilePath = filepath.Join(outFilePath, subfolder)
		}

		if err := os.MkdirAll(outFilePath, os.ModePerm); err != nil {
			panic(fmt.Errorf("creating output directory"))
		}
		outFilePath = filepath.Join(outFilePath, fmt.Sprintf("%s.%s.yaml", obj.GetName(), gk.Kind))

		outFile, err := os.Create(outFilePath)
		if err != nil {
			panic(fmt.Errorf("creating output file: %w", err))
		}
		defer outFile.Close()

		yamlBytes, err := yaml.Marshal(objToMarshal)
		if err != nil {
			panic(err)
		}

		packageNamespaceOverride := os.Getenv("PKO_PACKAGE_NAMESPACE_OVERRIDE")
		if len(packageNamespaceOverride) > 0 {
			logger.Info("replacing default package-operator-system namespace", "new ns", packageNamespaceOverride)
			yamlBytes = bytes.ReplaceAll(yamlBytes, []byte("package-operator-system"), []byte(packageNamespaceOverride))
		}

		if _, err := outFile.Write(yamlBytes); err != nil {
			panic(err)
		}
	}
}

func Deploy(ctx context.Context) {
	if _, ok := os.LookupEnv("VERSION"); ok {
		panic("VERSION environment variable not set, please set an explicit version to deploy")
	}

	cluster, err := dev.NewCluster(paths.ClusterDeploymentCache(), dev.WithKubeconfigPath(os.Getenv("KUBECONFIG")))
	if err != nil {
		panic(err)
	}

	var d Dev
	d.deployPackageOperatorManager(ctx, cluster)
	d.deployPackageOperatorWebhook(ctx, cluster)
}

// dumpManifestsFromFolder dumps all kubernets manifests from all files
// in the given folder into the output file. It does not recurse into subfolders.
// It dumps the manifests in lexical order based on file name.
func dumpManifestsFromFolder(folderPath string, outputPath string) {
	folder, err := os.Open(folderPath)
	if err != nil {
		panic(fmt.Errorf("open %q: %w", folderPath, err))
	}
	defer folder.Close()

	files, err := folder.Readdir(-1)
	if err != nil {
		panic(fmt.Errorf("reading directory: %w", err))
	}
	sort.Sort(fileInfosByName(files))

	if _, err = os.Stat(outputPath); err == nil {
		err = os.Remove(outputPath)
		if err != nil {
			panic(fmt.Errorf("removing old file: %s", err))
		}
	}

	outputFile, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		panic(fmt.Errorf("failed opening file: %s", err))
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
			panic(fmt.Errorf("reading %s: %w", filePath, err))
		}

		_, err = outputFile.Write(cleanFileYaml)
		if err != nil {
			panic(fmt.Errorf("failed appending manifest from file %s to output file: %s", file, err))
		}
		if i != len(files)-1 {
			_, err = outputFile.WriteString("\n---\n")
			if err != nil {
				panic(fmt.Errorf("failed appending --- %s to output file: %s", file, err))
			}
		} else {
			_, err = outputFile.WriteString("\n")
			if err != nil {
				panic(fmt.Errorf("failed appending new line %s to output file: %s", file, err))
			}
		}
	}
}

func templatePackageOperatorManager(scheme *k8sruntime.Scheme) (deploy *appsv1.Deployment) {
	objs, err := dev.LoadKubernetesObjectsFromFile(filepath.Join("config", "static-deployment", "deployment.yaml.tpl"))
	if err != nil {
		panic(fmt.Errorf("loading package-operator-manager deployment.yaml.tpl: %w", err))
	}

	return patchPackageOperatorManager(scheme, &objs[0])
}

func patchPackageOperatorManager(scheme *k8sruntime.Scheme, obj *unstructured.Unstructured) (deploy *appsv1.Deployment) {
	// Replace image
	packageOperatorDeployment := &appsv1.Deployment{}
	if err := scheme.Convert(
		obj, packageOperatorDeployment, nil); err != nil {
		panic(fmt.Errorf("converting to Deployment: %w", err))
	}

	var packageOperatorManagerImage string
	if len(os.Getenv("USE_DIGESTS")) > 0 {
		// To use digests the image needs to be pushed to a registry first.
		mg.Deps(mg.F(Builder.Push, "package-operator-manager"))
		packageOperatorManagerImage = Builder.imageURLWithDigest("package-operator-manager")
	} else {
		packageOperatorManagerImage = Builder.imageURL("package-operator-manager")
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

	return packageOperatorDeployment
}

// Testing and Linting
// -------------------
type Test mg.Namespace

// Runs linters.
func (Test) Lint() {
	mg.SerialDeps(Test.GolangCILint, Test.GoFmt, Test.GoImports, Test.GoModTidy)
}

func (Test) GolangCILint() {
	// Generate.All ensures code generators are re-triggered.
	mg.Deps(Generate.All, Dependency.GolangciLint)
	must(sh.RunV("golangci-lint", "run", "./...", "--deadline=15m"))
}

func (Test) GoFmt() {
	// Generate.All ensures code generators are re-triggered.
	mg.Deps(Generate.All)
	must(sh.RunV("go", "fmt", "./..."))
}

func (Test) GoModTidy() {
	// Generate.All ensures code generators are re-triggered.
	mg.Deps(Generate.All)
	must(sh.RunV("go", "mod", "tidy"))
}

func (Test) GoImports() {
	mg.Deps(Generate.All, Dependency.Goimports)
	must(sh.RunV("goimports", "-local", "package-operator.run", "-w", "-l", "."))
}

func (Test) ValidateGitClean() {
	// Generate.All ensures code generators are re-triggered.
	mg.Deps(Generate.All)

	o, err := sh.Output("git", "status", "--porcelain")
	must(err)

	if len(o) != 0 {
		panic("Repo is dirty! Probably because gofmt or make generate touched something...")
	}
}

// Runs unittests.
func (Test) Unit() {
	_, isCI := os.LookupEnv("CI")
	testCmd := fmt.Sprintf("go test -coverprofile=%s -race", paths.UnitTestCoverageReport())
	if isCI {
		// test output in json format
		testCmd += " -json"
	}
	testCmd += " ./internal/... ./cmd/..."

	if isCI {
		testCmd = testCmd + " > " + paths.UnitTestExecReport()
	}

	// cgo needed to enable race detector -race
	must(sh.RunWithV(map[string]string{"CGO_ENABLED": "1"}, "bash", "-c", testCmd))
}

// Runs PKO integration tests against whatever cluster your KUBECONFIG is pointing at.
func (t Test) Integration(ctx context.Context) { t.integration(ctx, "") }

// Runs PKO integration tests against whatever cluster your KUBECONFIG is pointing at.
// Also allows specifying only sub tests to run e.g. ./mage test:integrationrun TestPackage_success
func (t Test) IntegrationRun(ctx context.Context, filter string) { t.integration(ctx, filter) }

func (Test) integration(ctx context.Context, filter string) {
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
		args := []string{"export", "logs", paths.IntegrationTestLogs(), "--name", clusterName}
		if err := devEnvironment.RunKindCommand(ctx, os.Stdout, os.Stderr, args...); err != nil {
			logger.Error(err, "exporting logs")
		}
	}

	if testErr != nil {
		panic(testErr)
	}
}

// Building
// --------
type Build mg.Namespace

// Build all PKO binaries for the architecture of this machine.
func (Build) Binaries() {
	targets := []interface{}{mg.F(Builder.Cmd, "mage", "", "")}
	for name := range commands {
		targets = append(targets, mg.F(Builder.Cmd, name, nativeArch.OS, nativeArch.Arch))
	}

	mg.Deps(targets...)
}

func (Build) ReleaseBinaries() {
	targets := []interface{}{}
	for name, cmd := range commands {
		for _, arch := range cmd.ReleaseArchitectures {
			targets = append(targets, mg.F(Builder.Cmd, name, arch.OS, arch.Arch))
		}
	}
	mg.Deps(targets...)

	for name, cmd := range commands {
		for _, arch := range cmd.ReleaseArchitectures {
			dst := filepath.Join("bin", fmt.Sprintf("%s_%s_%s", name, arch.OS, arch.Arch))
			must(sh.Copy(dst, paths.binaryDst(name, arch)))
		}
	}
}

func (Build) Binary(cmd string) { mg.Deps(mg.F(Builder.Cmd, cmd, runtime.GOOS, runtime.GOARCH)) }

// Builds the given container image, building binaries as prerequisite as required.
func (Build) Image(image string) { mg.Deps(mg.F(Builder.Image, image)) }

// Builds all PKO container images.
func (Build) Images() {
	targets := []interface{}{}
	for name := range packageImages {
		targets = append(targets, mg.F(Builder.Image, name))
	}

	for name := range commandImages {
		targets = append(targets, mg.F(Builder.Image, name))
	}
	mg.Deps(targets...)
}

// Builds and pushes only the given container image to the default registry.
func (Build) PushImage(image string) { mg.Deps(mg.F(Builder.Push, image)) }

// Builds and pushes all container images to the default registry.
func (Build) PushImages() {
	targets := []interface{}{}
	for name := range packageImages {
		targets = append(targets, mg.F(Builder.Image, name))
	}

	for name := range commandImages {
		targets = append(targets, mg.F(Builder.Image, name))
	}
	mg.Deps(targets...)
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
	return paths.dependencies.GoInstall("controller-gen", url, controllerGenVersion)
}

func (d Dependency) Goimports() error {
	url := "golang.org/x/tools/cmd/goimports"
	return paths.dependencies.GoInstall("go-imports", url, goimportsVersion)
}

func (d Dependency) GolangciLint() error {
	url := "github.com/golangci/golangci-lint/cmd/golangci-lint"
	return paths.dependencies.GoInstall("golangci-lint", url, golangciLintVersion)
}

func (d Dependency) Docgen() error {
	url := "github.com/thetechnick/k8s-docgen"
	return paths.dependencies.GoInstall("k8s-docgen", url, k8sDocGenVersion)
}

// Ensure Kind dependency - Kubernetes in Docker (or Podman)
func (d Dependency) Kind() error {
	url := "sigs.k8s.io/kind"
	return paths.dependencies.GoInstall("kind", url, kindVersion)
}

// Builder
// -------

type builder struct {
	// Build Tags
	imageOrg string
	version  string
}

// init build variables
func (b *builder) init() {
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
}

// Builds binaries from /cmd directory.
func (b *builder) Cmd(cmd, goos, goarch string) {
	mg.SerialDeps(b.init)

	env := map[string]string{"CGO_ENABLED": "0"}

	bin := paths.binaryDst(cmd, nativeArch)
	if len(goos) != 0 || len(goarch) != 0 {
		bin = paths.binaryDst(cmd, archTarget{goos, goarch})
		env["GOOS"] = goos
		env["GOARCH"] = goarch
	}

	ldflags := "-w -s --extldflags '-zrelro -znow -O1'" + fmt.Sprintf("-X '%s/internal/version.version=%s'", module, b.version)
	cmdline := []string{"build", "--ldflags", ldflags, "--trimpath", "--mod=readonly", "-v", "-o", bin, "./cmd/" + cmd}

	if err := sh.RunWithV(env, "go", cmdline...); err != nil {
		panic(fmt.Errorf("compiling cmd/%s: %w", cmd, err))
	}
}

func (b *builder) Image(name string) {
	if strings.HasSuffix(name, "-package") {
		b.buildPackageImage(name)
	} else {
		b.buildCmdImage(name)
	}
}

// clean/prepare cache directory
func (b *builder) cleanImageCacheDir(name string) string {
	imageCacheDir := filepath.Join(paths.ImageCache(), name)
	if err := os.RemoveAll(imageCacheDir); err != nil && !os.IsNotExist(err) {
		panic(fmt.Errorf("deleting image cache: %w", err))
	}
	if err := os.Remove(imageCacheDir + ".tar"); err != nil && !os.IsNotExist(err) {
		panic(fmt.Errorf("deleting image cache: %w", err))
	}
	if err := os.MkdirAll(imageCacheDir, os.ModePerm); err != nil {
		panic(fmt.Errorf("create image cache dir: %w", err))
	}
	return imageCacheDir
}

// generic image build function, when the image just relies on
// a static binary build from cmd/*
func (b *builder) buildCmdImage(cmd string) {
	mg.SerialDeps(b.init, determineContainerRuntime, mg.F(b.Cmd, cmd, linuxAMD64Arch.OS, linuxAMD64Arch.Arch))

	imageCacheDir := b.cleanImageCacheDir(cmd)

	// prepare build context
	imageTag := b.imageURL(cmd)

	must(sh.Copy(filepath.Join(imageCacheDir, cmd), paths.binaryDst(cmd, linuxAMD64Arch)))
	must(sh.Copy(filepath.Join(imageCacheDir, "Containerfile"), filepath.Join("config", "images", cmd+".Containerfile")))
	must(sh.Copy(filepath.Join(imageCacheDir, "passwd"), filepath.Join("config", "images", "passwd")))

	cmds := [][]string{
		{containerRuntime, "build", "-t", imageTag, "-f", "Containerfile", "."},
		{containerRuntime, "image", "save", "-o", imageCacheDir + ".tar", imageTag},
	}

	// Build image!
	for _, command := range cmds {
		buildCmd := exec.Command(command[0], command[1:]...)
		buildCmd.Stderr = os.Stderr
		buildCmd.Stdout = os.Stdout
		buildCmd.Dir = imageCacheDir
		must(buildCmd.Run())
	}
}

func (b *builder) buildPackageImage(packageImageName string) {
	mg.SerialDeps(b.init, determineContainerRuntime)
	if packageImageName == "package-operator-package" {
		// inject digests into package
		mg.SerialDeps(Generate.PackageOperatorPackage)
	}

	imageCacheDir := b.cleanImageCacheDir(packageImageName)
	imageTag := b.imageURL(packageImageName)
	packageName := strings.TrimSuffix(packageImageName, "-package")

	// Copy files for build environment
	must(sh.Copy(filepath.Join(imageCacheDir, "Containerfile"), filepath.Join("config", "images", "package.Containerfile")))
	must(sh.Run("cp", "-a", filepath.Join("config/packages", packageName)+"/.", imageCacheDir+"/"))

	cmds := [][]string{
		{containerRuntime, "build", "-t", imageTag, "-f", "Containerfile", "."},
		{containerRuntime, "image", "save", "-o", imageCacheDir + ".tar", imageTag},
	}

	// Build image!
	for _, command := range cmds {
		buildCmd := exec.Command(command[0], command[1:]...)
		buildCmd.Stderr = os.Stderr
		buildCmd.Stdout = os.Stdout
		buildCmd.Dir = imageCacheDir
		if err := buildCmd.Run(); err != nil {
			panic(fmt.Errorf("running %q: %w", strings.Join(command, " "), err))
		}
	}
}

func (b *builder) Push(imageName string) {
	mg.SerialDeps(mg.F(b.Image, imageName))

	// Login to container registry when running on AppSRE Jenkins.
	_, isJenkins := os.LookupEnv("JENKINS_HOME")
	_, isCI := os.LookupEnv("CI")
	if isJenkins || isCI {
		log.Println("running in CI, calling container runtime login")
		args := []string{"login", "-u=" + os.Getenv("QUAY_USER"), "-p=" + os.Getenv("QUAY_TOKEN"), "quay.io"}
		if err := sh.Run(containerRuntime, args...); err != nil {
			panic(fmt.Errorf("registry login: %w", err))
		}
	}

	args := []string{"push"}
	if containerRuntime == string(dev.ContainerRuntimePodman) {
		args = append(args, "--digestfile="+paths.DigestFile(imageName))
	}
	args = append(args, b.imageURL(imageName))

	if err := sh.Run(containerRuntime, args...); err != nil {
		panic(fmt.Errorf("pushing image: %w", err))
	}
}

func (b *builder) imageURL(name string) string { return b.internalImageURL(name, false) }

func (b *builder) imageURLWithDigest(name string) string { return b.internalImageURL(name, true) }

func (b *builder) internalImageURL(name string, useDigest bool) string {
	envvar := strings.ReplaceAll(strings.ToUpper(name), "-", "_") + "_IMAGE"
	if url := os.Getenv(envvar); len(url) != 0 {
		return url
	}
	image := b.imageOrg + "/" + name + ":" + b.version
	if !useDigest {
		return image
	}

	digest, err := os.ReadFile(paths.DigestFile(name))
	if err != nil {
		panic(err)
	}

	return b.imageOrg + "/" + name + "@" + string(digest)
}

// Development
// -----------

type Dev mg.Namespace

// Creates an empty development environment via kind.
func (d Dev) Setup(ctx context.Context) {
	mg.SerialDeps(Dev.init)

	if err := devEnvironment.Init(ctx); err != nil {
		panic(fmt.Errorf("initializing dev environment: %w", err))
	}
}

// Tears the whole kind development environment down.
func (d Dev) Teardown(ctx context.Context) {
	mg.SerialDeps(Dev.init)

	if err := devEnvironment.Destroy(ctx); err != nil {
		panic(fmt.Errorf("tearing down dev environment: %w", err))
	}
}

// Load images into the development environment.
func (d Dev) Load() {
	// setup is a pre-requisite and needs to run before we can load images.
	mg.SerialDeps(Dev.Setup)
	images := []string{
		"package-operator-manager", "package-operator-webhook",
		"remote-phase-manager", "test-stub", "test-stub-package",
		"package-operator-package",
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
func (d Dev) Deploy(ctx context.Context) {
	mg.SerialDeps(Dev.Load)

	d.deployPackageOperatorManager(ctx, devEnvironment.Cluster)
	d.deployPackageOperatorWebhook(ctx, devEnvironment.Cluster)
	d.deployRemotePhaseManager(ctx, devEnvironment.Cluster)
}

// deploy the Package Operator Manager from local files.
func (d Dev) deployPackageOperatorManager(ctx context.Context, cluster *dev.Cluster) {
	packageOperatorDeployment := templatePackageOperatorManager(cluster.Scheme)

	ctx = logr.NewContext(ctx, logger)

	// Deploy
	if err := cluster.CreateAndWaitFromFolders(ctx, []string{filepath.Join("config", "static-deployment")}); err != nil {
		panic(fmt.Errorf("deploy package-operator-manager dependencies: %w", err))
	}
	_ = cluster.CtrlClient.Delete(ctx, packageOperatorDeployment)
	if err := cluster.CreateAndWaitForReadiness(ctx, packageOperatorDeployment); err != nil {
		panic(fmt.Errorf("deploy package-operator-manager: %w", err))
	}
}

// Package Operator Webhook server from local files.
func (d Dev) deployPackageOperatorWebhook(ctx context.Context, cluster *dev.Cluster) {
	objs, err := dev.LoadKubernetesObjectsFromFile(filepath.Join("config", "deploy", "webhook", "deployment.yaml.tpl"))
	if err != nil {
		panic(fmt.Errorf("loading package-operator-webhook deployment.yaml.tpl: %w", err))
	}

	// Replace image
	packageOperatorWebhookDeployment := &appsv1.Deployment{}
	if err := cluster.Scheme.Convert(
		&objs[0], packageOperatorWebhookDeployment, nil); err != nil {
		panic(fmt.Errorf("converting to Deployment: %w", err))
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
		filepath.Join("config", "deploy", "webhook", "00-tls-secret.yaml"),
		filepath.Join("config", "deploy", "webhook", "service.yaml.tpl"),
		filepath.Join("config", "deploy", "webhook", "objectsetvalidatingwebhookconfig.yaml"),
		filepath.Join("config", "deploy", "webhook", "objectsetphasevalidatingwebhookconfig.yaml"),
		filepath.Join("config", "deploy", "webhook", "clusterobjectsetvalidatingwebhookconfig.yaml"),
		filepath.Join("config", "deploy", "webhook", "clusterobjectsetphasevalidatingwebhookconfig.yaml"),
	}); err != nil {
		panic(fmt.Errorf("deploy package-operator-webhook dependencies: %w", err))
	}
	_ = cluster.CtrlClient.Delete(ctx, packageOperatorWebhookDeployment)
	if err := cluster.CreateAndWaitForReadiness(ctx, packageOperatorWebhookDeployment); err != nil {
		panic(fmt.Errorf("deploy package-operator-webhook: %w", err))
	}
}

// Remote phase manager from local files.
func (d Dev) deployRemotePhaseManager(ctx context.Context, cluster *dev.Cluster) {
	objs, err := dev.LoadKubernetesObjectsFromFile(filepath.Join("config", "remote-phase-static-deployment", "deployment.yaml.tpl"))
	if err != nil {
		panic(fmt.Errorf("loading package-operator-webhook deployment.yaml.tpl: %w", err))
	}

	// Insert new image in remote-phase-manager deployment manifest
	remotePhaseManagerDeployment := &appsv1.Deployment{}
	if err := cluster.Scheme.Convert(&objs[0], remotePhaseManagerDeployment, nil); err != nil {
		panic(fmt.Errorf("converting to Deployment: %w", err))
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
	err = cluster.CreateAndWaitFromFolders(ctx, []string{filepath.Join("config", "remote-phase-static-deployment")})
	if err != nil {
		panic(fmt.Errorf("deploy remote-phase-manager dependencies: %w", err))
	}

	// Get Kubeconfig, will be edited for the target service account
	targetKubeconfigPath := os.Getenv("TARGET_KUBECONFIG_PATH")
	var kubeconfigBytes []byte
	if len(targetKubeconfigPath) == 0 {
		kubeconfigBuf := new(bytes.Buffer)
		args := []string{"get", "kubeconfig", "--name", clusterName, "--internal"}
		err = devEnvironment.RunKindCommand(ctx, kubeconfigBuf, os.Stderr, args...)
		if err != nil {
			panic(fmt.Errorf("exporting internal kubeconfig: %w", err))
		}
		kubeconfigBytes = kubeconfigBuf.Bytes()
		old := []byte("package-operator-dev-control-plane:6443")
		new := []byte("kubernetes.default")
		kubeconfigBytes = bytes.Replace(kubeconfigBytes, old, new, -1) // use in-cluster DNS
	} else {
		kubeconfigBytes, err = ioutil.ReadFile(targetKubeconfigPath)
		if err != nil {
			panic(fmt.Errorf("reading in kubeconfig: %w", err))
		}
	}

	kubeconfigMap := map[string]interface{}{}
	err = yaml.UnmarshalStrict(kubeconfigBytes, &kubeconfigMap)
	if err != nil {
		panic(fmt.Errorf("unmarshalling kubeconfig: %w", err))
	}

	// Get target cluster service account
	targetSASecret := &corev1.Secret{}
	key := client.ObjectKey{Namespace: "package-operator-system", Name: "remote-phase-operator-target-cluster"}
	err = cluster.CtrlClient.Get(context.TODO(), key, targetSASecret)
	if err != nil {
		panic(fmt.Errorf("reading in service account secret: %w", err))
	}

	// Insert target cluster service account token into kubeconfig
	kubeconfigMap["users"] = []map[string]interface{}{
		{"name": "kind-package-operator-dev", "user": map[string]string{"token": string(targetSASecret.Data["token"])}},
	}

	newKubeconfigBytes, err := yaml.Marshal(kubeconfigMap)
	if err != nil {
		panic(fmt.Errorf("marshalling new kubeconfig back to yaml: %w", err))
	}

	// Create a new secret for the kubeconfig
	secret := &corev1.Secret{}
	objs, err = dev.LoadKubernetesObjectsFromFile(filepath.Join("config", "remote-phase-static-deployment", "2-secret.yaml.tpl"))
	if err != nil {
		panic(fmt.Errorf("loading package-operator-webhook 2-secret.yaml.tpl: %w", err))
	}
	if err := cluster.Scheme.Convert(&objs[0], secret, nil); err != nil {
		panic(fmt.Errorf("converting to Secret: %w", err))
	}

	// insert the new kubeconfig into the secret
	secret.Data = map[string][]byte{"kubeconfig": newKubeconfigBytes}

	ctx = logr.NewContext(ctx, logger)

	// Deploy the secret with the new kubeconfig
	_ = cluster.CtrlClient.Delete(ctx, secret)
	if err := cluster.CreateAndWaitForReadiness(ctx, secret); err != nil {
		panic(fmt.Errorf("deploy kubeconfig secret: %w", err))
	}
	// Deploy the remote phase manager deployment
	_ = cluster.CtrlClient.Delete(ctx, remotePhaseManagerDeployment)
	if err := cluster.CreateAndWaitForReadiness(ctx, remotePhaseManagerDeployment); err != nil {
		panic(fmt.Errorf("deploy remote-phase-manager: %w", err))
	}
}

// Setup local dev environment with the package operator installed and run the integration test suite.
func (d Dev) Integration(ctx context.Context) {
	mg.SerialDeps(Dev.Deploy)

	os.Setenv("KUBECONFIG", devEnvironment.Cluster.Kubeconfig())

	mg.SerialDeps(Test.Integration)
}

func (d Dev) LoadImage(ctx context.Context, image string) {
	mg.Deps(mg.F(Build.Image, image))

	imageTar := filepath.Join(paths.ImageCache(), image+".tar")
	if err := devEnvironment.LoadImageFromTar(ctx, imageTar); err != nil {
		panic(fmt.Errorf("load image from tar: %w", err))
	}
}

func (d Dev) init() {
	mg.SerialDeps(determineContainerRuntime, Dependency.Kind)

	devEnvironment = dev.NewEnvironment(
		clusterName,
		paths.DevCache(),
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
// installYamlFile has to come after code generation
func (Generate) All() { mg.SerialDeps(Generate.code, Generate.docs, Generate.installYamlFile) }

func (Generate) code() {
	mg.Deps(Dependency.ControllerGen)

	args := []string{"crd:crdVersions=v1,generateEmbeddedObjectMeta=true", "paths=./core/...", "output:crd:artifacts:config=../config/crds"}
	manifestsCmd := exec.Command("controller-gen", args...)
	manifestsCmd.Dir = paths.APISubmodule()
	manifestsCmd.Stdout = os.Stdout
	manifestsCmd.Stderr = os.Stderr
	if err := manifestsCmd.Run(); err != nil {
		panic(fmt.Errorf("generating kubernetes manifests: %w", err))
	}

	// code gen
	codeCmd := exec.Command("controller-gen", "object", "paths=./...")
	codeCmd.Dir = paths.APISubmodule()
	if err := codeCmd.Run(); err != nil {
		panic(fmt.Errorf("generating deep copy methods: %w", err))
	}

	crds, err := filepath.Glob(filepath.Join("config", "crds", "*.yaml"))
	if err != nil {
		panic(fmt.Errorf("finding CRDs: %w", err))
	}

	for _, crd := range crds {
		cmd := []string{"cp", crd, filepath.Join("config", "static-deployment", "1-"+filepath.Base(crd))}
		if err := sh.RunV(cmd[0], cmd[1:]...); err != nil {
			panic(fmt.Errorf("running %q: %w", strings.Join(cmd, " "), err))
		}
	}
}

func (Generate) docs() {
	mg.Deps(Dependency.Docgen)

	_, isCI := os.LookupEnv("CI")
	testCmd := fmt.Sprintf("go test -coverprofile=%s -race", paths.UnitTestCoverageReport())
	if isCI {
		// test output in json format
		testCmd += " -json"
	}
	testCmd += " ./internal/... ./cmd/..."

	if isCI {
		testCmd = testCmd + " > " + paths.UnitTestExecReport()
	}

	// cgo needed to enable race detector -race
	must(sh.RunWithV(map[string]string{"CGO_ENABLED": "1"}, "bash", "-c", testCmd))

	refPath := paths.APIReference()
	// Move the hack script in here.
	must(sh.RunV("bash", "-c", fmt.Sprintf("k8s-docgen apis/core/v1alpha1 > %s", refPath)))
	must(sh.RunV("bash", "-c", fmt.Sprintf("echo >> %s", refPath)))
	must(sh.RunV("bash", "-c", fmt.Sprintf("k8s-docgen apis/manifests/v1alpha1 >> %s", refPath)))
	must(sh.RunV("bash", "-c", fmt.Sprintf("echo >> %s", refPath)))
}

func (Generate) installYamlFile() {
	dumpManifestsFromFolder(filepath.Join("config", "static-deployment"), "install.yaml")
}

// Sorts fs.FileInfo objects by basename.
type fileInfosByName []fs.FileInfo

func (x fileInfosByName) Len() int { return len(x) }

func (x fileInfosByName) Less(i, j int) bool {
	return filepath.Base(x[i].Name()) < filepath.Base(x[j].Name())
}

func (x fileInfosByName) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

// Includes all static-deployment files in the package-operator-package.
func (Generate) PackageOperatorPackage() error {
	return filepath.WalkDir("config/static-deployment", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		includeInPackageOperatorPackage(path)
		return nil
	})
}

// generates a self-bootstrap-job.yaml based on the current VERSION.
// requires the images to have been build beforehand.
func (Generate) SelfBootstrapJob() {
	mg.Deps(determineContainerRuntime)

	const (
		pkoDefaultManagerImage = "quay.io/package-operator/package-operator-manager:latest"
		pkoDefaultPackageImage = "quay.io/package-operator/package-operator-package:latest"
	)

	latestJob, err := os.ReadFile("config/self-bootstrap-job.yaml.tpl")
	if err != nil {
		panic(err)
	}

	var (
		packageOperatorManagerImage string
		packageOperatorPackageImage string
	)
	if len(os.Getenv("USE_DIGESTS")) > 0 {
		mg.Deps(mg.F(Builder.Push, "package-operator-manager"), mg.F(Builder.Push, "package-operator-package"))
		packageOperatorManagerImage = Builder.imageURLWithDigest("package-operator-manager")
		packageOperatorPackageImage = Builder.imageURLWithDigest("package-operator-package")
	} else {
		packageOperatorManagerImage = Builder.imageURL("package-operator-manager")
		packageOperatorPackageImage = Builder.imageURL("package-operator-package")
	}

	latestJob = bytes.ReplaceAll(latestJob, []byte(pkoDefaultManagerImage), []byte(packageOperatorManagerImage))
	latestJob = bytes.ReplaceAll(latestJob, []byte(pkoDefaultPackageImage), []byte(packageOperatorPackageImage))

	must(os.WriteFile("config/self-bootstrap-job.yaml", latestJob, os.ModePerm))
}
