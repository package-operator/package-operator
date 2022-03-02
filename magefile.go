//go:build mage
// +build mage

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/mt-sre/devkube/dev"
	"github.com/mt-sre/devkube/magedeps"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

const (
	module                  = "github.com/openshift/addon-operator"
	defaultImageOrg         = "quay.io/app-sre"
	defaultContainerRuntime = "podman"
)

// Directories
var (
	// Working directory of the project.
	workDir string
	// Dependency directory.
	depsDir  magedeps.DependencyDirectory
	cacheDir string

	logger *logr.Logger
)

// Building
// --------
type Build mg.Namespace

// Build Tags
var (
	branch        string
	shortCommitID string
	version       string
	buildDate     string

	ldFlags string

	imageOrg         string
	containerRuntime string
)

// init build variables
func (Build) init() error {
	// Build flags
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchBytes, err := branchCmd.Output()
	if err != nil {
		panic(fmt.Errorf("getting git branch: %w", err))
	}
	branch = strings.TrimSpace(string(branchBytes))

	shortCommitIDCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	shortCommitIDBytes, err := shortCommitIDCmd.Output()
	if err != nil {
		panic(fmt.Errorf("getting git short commit id"))
	}
	shortCommitID = strings.TrimSpace(string(shortCommitIDBytes))

	version = strings.TrimSpace(os.Getenv("VERSION"))
	if len(version) == 0 {
		version = shortCommitID
	}

	buildDate = fmt.Sprint(time.Now().UTC().Unix())
	ldFlags = fmt.Sprintf(`-X %s/internal/version.Version=%s`+
		`-X %s/internal/version.Branch=%s`+
		`-X %s/internal/version.Commit=%s`+
		`-X %s/internal/version.BuildDate=%s`,
		module, version,
		module, branch,
		module, shortCommitID,
		module, buildDate,
	)

	imageOrg = os.Getenv("IMAGE_ORG")
	if len(imageOrg) == 0 {
		imageOrg = defaultImageOrg
	}

	containerRuntime = os.Getenv("CONTAINER_RUNTIME")
	if len(containerRuntime) == 0 {
		containerRuntime = defaultContainerRuntime
	}

	return nil
}

// Builds binaries from /cmd directory.
func (Build) cmd(cmd, goos, goarch string) error {
	mg.Deps(Build.init)

	env := map[string]string{
		"GOFLAGS":     "",
		"CGO_ENABLED": "0",
		"LDFLAGS":     ldFlags,
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

// Default build target for CI/CD
func (Build) All() {
	mg.Deps(
		mg.F(Build.cmd, "addon-operator-manager", "linux", "amd64"),
		mg.F(Build.cmd, "addon-operator-webhook", "linux", "amd64"),
		mg.F(Build.cmd, "api-mock", "linux", "amd64"),
		mg.F(Build.cmd, "mage", "", ""),
	)
}

func (Build) BuildImages() {
	mg.Deps(
		mg.F(Build.ImageBuild, "addon-operator-manager"),
		mg.F(Build.ImageBuild, "addon-operator-webhook"),
		mg.F(Build.ImageBuild, "api-mock"),
		mg.F(Build.ImageBuild, "addon-operator-index"), // also pushes bundle
	)
}

func (Build) PushImages() {
	mg.Deps(
		mg.F(Build.imagePush, "addon-operator-manager"),
		mg.F(Build.imagePush, "addon-operator-webhook"),
		mg.F(Build.imagePush, "addon-operator-index"), // also pushes bundle
	)
}

// Builds the docgen internal tool
func (Build) Docgen() {
	mg.Deps(mg.F(Build.cmd, "docgen", "", ""))
}

func (b Build) ImageBuild(cmd string) error {
	// clean/prepare cache directory
	imageCacheDir := path.Join(cacheDir, "image", cmd)
	if err := os.RemoveAll(imageCacheDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting image cache: %w", err)
	}
	if err := os.Remove(imageCacheDir + ".tar"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting image cache: %w", err)
	}
	if err := os.MkdirAll(imageCacheDir, os.ModePerm); err != nil {
		return fmt.Errorf("create image cache dir: %w", err)
	}

	switch cmd {
	case "addon-operator-index":
		return b.buildOLMIndexImage(imageCacheDir)

	case "addon-operator-bundle":
		return b.buildOLMBundleImage(imageCacheDir)

	default:
		mg.Deps(
			mg.F(Build.cmd, cmd, "linux", "amd64"),
		)
		return b.buildGenericImage(cmd, imageCacheDir)
	}
}

// generic image build function, when the image just relies on
// a static binary build from cmd/*
func (Build) buildGenericImage(cmd, imageCacheDir string) error {
	imageTag := imageURL(cmd)
	for _, command := range [][]string{
		// Copy files for build environment
		{"cp", "-a",
			"bin/linux_amd64/" + cmd,
			imageCacheDir + "/" + cmd},
		{"cp", "-a",
			"config/docker/" + cmd + ".Dockerfile",
			imageCacheDir + "/Dockerfile"},

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

func (b Build) buildOLMIndexImage(imageCacheDir string) error {
	mg.Deps(
		Dependency.Opm,
		mg.F(Build.imagePush, "addon-operator-bundle"),
	)

	if err := sh.RunV("opm", "index", "add",
		"--container-tool", containerRuntime,
		"--bundles", imageURL("addon-operator-bundle"),
		"--tag", imageURL("addon-operator-index")); err != nil {
		return fmt.Errorf("runnign opm: %w", err)
	}
	return nil
}

func (b Build) buildOLMBundleImage(imageCacheDir string) error {
	mg.Deps(
		Build.init,
		Build.TemplateAddonOperatorCSV,
	)

	imageTag := imageURL("addon-operator-bundle")
	manifestsDir := path.Join(imageCacheDir, "manifests")
	metadataDir := path.Join(imageCacheDir, "metadata")
	for _, command := range [][]string{
		{"mkdir", "-p", manifestsDir},
		{"mkdir", "-p", metadataDir},

		// Copy files for build environment
		{"cp", "-a",
			"config/docker/addon-operator-bundle.Dockerfile",
			imageCacheDir + "/Dockerfile"},

		{"cp", "-a", "config/olm/addon-operator.csv.yaml", manifestsDir},
		{"cp", "-a", "config/olm/metrics.service.yaml", manifestsDir},
		{"cp", "-a", "config/olm/annotations.yaml", metadataDir},

		// copy CRDs
		// The first few lines of the CRD file need to be removed:
		// https://github.com/operator-framework/operator-registry/issues/222
		{"bash", "-c", "tail -n+3 " +
			"config/deploy/addons.managed.openshift.io_addons.yaml " +
			"> " + path.Join(manifestsDir, "addons.yaml")},
		{"bash", "-c", "tail -n+3 " +
			"config/deploy/addons.managed.openshift.io_addonoperators.yaml " +
			"> " + path.Join(manifestsDir, "addonoperators.yaml")},
		{"bash", "-c", "tail -n+3 " +
			"config/deploy/addons.managed.openshift.io_addoninstances.yaml " +
			"> " + path.Join(manifestsDir, "addoninstances.yaml")},

		// Build image!
		{containerRuntime, "build", "-t", imageTag, imageCacheDir},
		{containerRuntime, "image", "save",
			"-o", imageCacheDir + ".tar", imageTag},
	} {
		if err := sh.RunV(command[0], command[1:]...); err != nil {
			return err
		}
	}
	return nil
}

func (b Build) TemplateAddonOperatorCSV() error {
	objs, err := dev.LoadKubernetesObjectsFromFile(
		"config/olm/addon-operator.csv.tpl.yaml")
	if err != nil {
		return fmt.Errorf("loading CSV template: %w", err)
	}
	if len(objs) != 1 {
		return fmt.Errorf(
			"loaded %d kube objects from CSV template, expected 1",
			len(objs))
	}

	// convert unstructured.Unstructured to CSV
	scheme := runtime.NewScheme()
	if err := operatorsv1alpha1.AddToScheme(scheme); err != nil {
		return err
	}
	var csv operatorsv1alpha1.ClusterServiceVersion
	if err := scheme.Convert(&objs[0], &csv, nil); err != nil {
		return err
	}

	// replace images
	for i := range csv.Spec.
		InstallStrategy.StrategySpec.DeploymentSpecs {
		deploy := &csv.Spec.
			InstallStrategy.StrategySpec.DeploymentSpecs[i]

		switch deploy.Name {
		case "addon-operator-manager":
			for i := range deploy.Spec.
				Template.Spec.Containers {
				container := &deploy.Spec.Template.Spec.Containers[i]
				switch container.Name {
				case "manager":
					container.Image = imageURL("addon-operator-manager")
				}
			}

		case "addon-operator-webhook":
			for i := range deploy.Spec.
				Template.Spec.Containers {
				container := &deploy.Spec.Template.Spec.Containers[i]
				switch container.Name {
				case "webhook":
					container.Image = imageURL("addon-operator-webhook")
				}
			}
		}
	}
	csv.Annotations["containerImage"] = imageURL("addon-operator-manager")

	// write
	csvBytes, err := yaml.Marshal(csv)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile("config/olm/addon-operator.csv.yaml",
		csvBytes, os.ModePerm); err != nil {
		return err
	}

	return nil
}

func (Build) imagePush(imageName string) error {
	mg.Deps(
		mg.F(Build.ImageBuild, imageName),
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

	if err := sh.Run(containerRuntime, "push", imageURL(imageName)); err != nil {
		return fmt.Errorf("pushing image: %w", err)
	}

	return nil
}

func imageURL(name string) string {
	return imageOrg + "/" + name + ":" + version
}

// Code Generators
// ---------------
type Generate mg.Namespace

func (Generate) All() {
	mg.Deps(
		Generate.code,
		Generate.docs,
	)
}

func (Generate) code() error {
	mg.Deps(Dependency.ControllerGen)

	manifestsCmd := exec.Command("controller-gen",
		"crd:crdVersions=v1", "rbac:roleName=addon-operator-manager",
		"paths=./...", "output:crd:artifacts:config=../config/deploy")
	manifestsCmd.Dir = workDir + "/apis"
	if err := manifestsCmd.Run(); err != nil {
		return fmt.Errorf("generating kubernetes manifests: %w", err)
	}

	// code gen
	codeCmd := exec.Command("controller-gen", "object", "paths=./...")
	codeCmd.Dir = workDir + "/apis"
	if err := codeCmd.Run(); err != nil {
		return fmt.Errorf("generating deep copy methods: %w", err)
	}

	// patching generated code to stay go 1.16 output compliant
	// https://golang.org/doc/go1.17#gofmt
	// @TODO: remove this when we move to go 1.17"
	// otherwise our ci will fail because of changed files"
	// this removes the line '//go:build !ignore_autogenerated'"
	findArgs := []string{".", "-name", "zz_generated.deepcopy.go", "-exec",
		"sed", "-i", `/\/\/go:build !ignore_autogenerated/d`, "{}", ";"}

	// The `-i` flag works a bit differenly on MacOS (I don't know why.)
	// See - https://stackoverflow.com/a/19457213
	if goruntime.GOOS == "darwin" {
		findArgs = []string{".", "-name", "zz_generated.deepcopy.go", "-exec",
			"sed", "-i", "", "-e", `/\/\/go:build !ignore_autogenerated/d`, "{}", ";"}
	}
	if err := sh.Run("find", findArgs...); err != nil {
		return fmt.Errorf("removing go:build annotation: %w", err)
	}

	return nil
}

func (Generate) docs() error {
	mg.Deps(Build.Docgen)

	return sh.Run("./hack/docgen.sh")
}

// Testing and Linting
// -------------------
type Test mg.Namespace

func (Test) Lint() error {
	mg.Deps(
		Dependency.GolangciLint,
		Generate.All,
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

// Dependencies
// ------------

// Dependency Versions
const (
	controllerGenVersion = "0.6.2"
	kindVersion          = "0.11.1"
	yqVersion            = "4.12.0"
	goimportsVersion     = "0.1.5"
	golangciLintVersion  = "1.43.0"
	olmVersion           = "0.19.1"
	opmVersion           = "1.18.0"
	helmVersion          = "3.7.2"
)

type Dependency mg.Namespace

func (d Dependency) All() {
	mg.Deps(
		Dependency.Kind,
		Dependency.ControllerGen,
		Dependency.YQ,
		Dependency.Goimports,
		Dependency.GolangciLint,
		Dependency.Helm,
		Dependency.Opm,
	)
}

// Ensure Kind dependency - Kubernetes in Docker (or Podman)
func (d Dependency) Kind() error {
	return depsDir.GoInstall("kind",
		"sigs.k8s.io/kind", kindVersion)
}

// Ensure controller-gen - kubebuilder code and manifest generator.
func (d Dependency) ControllerGen() error {
	return depsDir.GoInstall("controller-gen",
		"sigs.k8s.io/controller-tools/cmd/controller-gen", controllerGenVersion)
}

// Ensure yq - jq but for Yaml, written in Go.
func (d Dependency) YQ() error {
	return depsDir.GoInstall("yq",
		"github.com/mikefarah/yq/v4", yqVersion)
}

func (d Dependency) Goimports() error {
	return depsDir.GoInstall("go-imports",
		"golang.org/x/tools/cmd/goimports", goimportsVersion)
}

func (d Dependency) GolangciLint() error {
	return depsDir.GoInstall("golangci-lint",
		"github.com/golangci/golangci-lint/cmd/golangci-lint", golangciLintVersion)
}

func (d Dependency) Helm() error {
	return depsDir.GoInstall("helm", "helm.sh/helm/v3/cmd/helm", helmVersion)
}

func (d Dependency) Opm() error {
	// TODO: move this into devkube library, to ensure the depsDir is present, even if you just call "NeedsRebuild"
	if err := os.MkdirAll(depsDir.Bin(), os.ModePerm); err != nil {
		return fmt.Errorf("create dependency dir: %w", err)
	}

	needsRebuild, err := depsDir.NeedsRebuild("opm", opmVersion)
	if err != nil {
		return err
	}
	if !needsRebuild {
		return nil
	}

	// Tempdir
	tempDir, err := os.MkdirTemp(cacheDir, "")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download
	tempOPMBin := path.Join(tempDir, "opm")
	if err := sh.RunV(
		"curl", "-L", "--fail",
		"-o", tempOPMBin,
		fmt.Sprintf(
			"https://github.com/operator-framework/operator-registry/releases/download/v%s/linux-amd64-opm",
			opmVersion,
		),
	); err != nil {
		return fmt.Errorf("downloading opm: %w", err)
	}

	if err := os.Chmod(tempOPMBin, 0755); err != nil {
		return fmt.Errorf("make opm executable: %w", err)
	}

	// Move
	if err := os.Rename(tempOPMBin, path.Join(depsDir.Bin(), "opm")); err != nil {
		return fmt.Errorf("move opm: %w", err)
	}
	return nil
}

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

}
