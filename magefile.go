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
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/mt-sre/devkube/dev"
	"github.com/mt-sre/devkube/magedeps"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientScheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"
)

// Constants that define build behaviour.
const (
	module                 = "package-operator.run/package-operator"
	defaultImageOrg        = "quay.io/package-operator"
	clusterName            = "package-operator-dev"
	cliCmdName             = "kubectl-package"
	pkoPackageName         = "package-operator-package"
	remotePhasePackageName = "remote-phase-package"

	controllerGenVersion = "0.6.2"
	golangciLintVersion  = "1.50.1"
	kindVersion          = "0.16.0"
	k8sDocGenVersion     = "0.5.1"
)

// Variables that define build behaviour.
var (
	// commands defines which commands under ./cmd shall be build and what architectures are
	// released.
	commands = map[string]*command{
		"package-operator-manager": {nil},
		"remote-phase-manager":     {nil},
		cliCmdName:                 {[]archTarget{linuxAMD64Arch, {"darwin", "amd64"}, {"darwin", "arm64"}}},
	}

	// packageImages defines what packages in this repository shall be build.
	// Note that you can't reference the Generate mage target in ExtraDeps
	// since that would result in a circulat dependency. They must be added via init() for now.
	packageImages = map[string]*PackageImage{
		pkoPackageName:         {Push: true, SourcePath: filepath.Join("config", "packages", "package-operator")},
		remotePhasePackageName: {Push: true, SourcePath: filepath.Join("config", "packages", "remote-phase")},
		"test-stub-package":    {SourcePath: filepath.Join("config", "packages", "test-stub")},
	}

	// commandImages defines what commands under ./cmd shall be packaged into images.
	commandImages = map[string]*CommandImage{
		"package-operator-manager": {Push: true},
		"package-operator-webhook": {Push: true},
		"remote-phase-manager":     {Push: true},
		"cli":                      {Push: true, BinaryName: "kubectl-package"},
		"test-stub":                {},
	}
)

// Variables that are automatically set and should not be touched.
var (
	nativeArch                     = archTarget{runtime.GOOS, runtime.GOARCH}
	linuxAMD64Arch                 = archTarget{"linux", "amd64"}
	locations                      = newLocations()
	logger             logr.Logger = stdr.New(nil)
	applicationVersion string
)

// Types for target configuration.
type (
	archTarget struct{ OS, Arch string }
	command    struct{ ReleaseArchitectures []archTarget }
	Locations  struct {
		lock             *sync.Mutex
		devEnvironment   *dev.Environment
		containerRuntime string
		cache            string
		imageOrg         string
	}
	CommandImage struct {
		Push       bool
		BinaryName string
	}
	PackageImage struct {
		ExtraDeps  []interface{}
		Push       bool
		SourcePath string
	}
	fileInfosByName []fs.FileInfo
)

// All the mage subtargets.
type (
	Test       mg.Namespace
	Build      mg.Namespace
	Dependency mg.Namespace
	Dev        mg.Namespace
	Generate   mg.Namespace
)

// Initialize all the global variables.
func init() {
	// Use a local directory to get around permission errors in OpenShift CI.
	os.Setenv("GOLANGCI_LINT_CACHE", filepath.Join(locations.Cache(), "golangci-lint"))
	os.Setenv("PATH", locations.Deps().Bin()+":"+os.Getenv("PATH"))

	// Extra dependencies must be specified here to avoid a circular dependency.
	packageImages[pkoPackageName].ExtraDeps = []interface{}{Generate.PackageOperatorPackage}
	packageImages[remotePhasePackageName].ExtraDeps = []interface{}{Generate.RemotePhasePackage}
}

// Must panics if the given error is not nil.
func must(err error) {
	if err != nil {
		panic(err)
	}
}

func newLocations() Locations {
	// Entrypoint ./mage uses .cache/magefile as cache so .cache should exist.
	absCache, err := filepath.Abs(".cache")
	must(err)

	// Use version from VERSION env if present, use "git describe" elsewise.
	applicationVersion = strings.TrimSpace(os.Getenv("VERSION"))
	if len(applicationVersion) == 0 {
		gitDescribeCmd := exec.Command("git", "describe", "--tags")
		byteVersion, err := gitDescribeCmd.Output()
		if err != nil {
			panic(fmt.Errorf("git describe: %w", err))
		}
		applicationVersion = strings.TrimSpace(string(byteVersion))
	}

	// image org
	imageOrg := os.Getenv("IMAGE_ORG")
	if len(imageOrg) == 0 {
		imageOrg = defaultImageOrg
	}

	l := Locations{lock: &sync.Mutex{}, cache: absCache, imageOrg: imageOrg}

	must(os.MkdirAll(string(l.Deps()), 0o755))
	must(os.MkdirAll(l.unitTestCache(), 0o755))

	return l
}

func includeInPackageOperatorPackage(file string, outDir string) {
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

			var deploy *appsv1.Deployment
			if obj.GetName() == "package-operator-remote-phase-manager" {
				deploy = patchRemotePhaseManager(clientScheme.Scheme, &obj)
				deploy.SetNamespace("")
			} else {
				deploy = patchPackageOperatorManager(clientScheme.Scheme, &obj)
			}
			deploy.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
			objToMarshal = deploy
		}
		obj.SetAnnotations(annotations)

		outFilePath := outDir
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

	cluster, err := dev.NewCluster(locations.ClusterDeploymentCache(), dev.WithKubeconfigPath(os.Getenv("KUBECONFIG")))
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
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		panic(fmt.Errorf("read dir %q: %w", folderPath, err))
	}

	infoByName := map[string]fs.DirEntry{}
	names := []string{}
	for _, i := range entries {
		names = append(names, i.Name())
		infoByName[i.Name()] = i
	}

	sort.Strings(names)

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
	for i, name := range names {
		if infoByName[name].IsDir() {
			continue
		}

		filePath := filepath.Join(folderPath, name)
		fileYaml, err := os.ReadFile(filePath)
		cleanFileYaml := bytes.Trim(fileYaml, "-\n")
		if err != nil {
			panic(fmt.Errorf("reading %s: %w", filePath, err))
		}

		_, err = outputFile.Write(cleanFileYaml)
		if err != nil {
			panic(fmt.Errorf("failed appending manifest from file %s to output file: %s", name, err))
		}
		if i != len(names)-1 {
			_, err = outputFile.WriteString("\n---\n")
			if err != nil {
				panic(fmt.Errorf("failed appending --- %s to output file: %s", name, err))
			}
		} else {
			_, err = outputFile.WriteString("\n")
			if err != nil {
				panic(fmt.Errorf("failed appending new line %s to output file: %s", name, err))
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

	var (
		packageOperatorManagerImage string
		remotePhasePackageImage     string
	)
	if len(os.Getenv("USE_DIGESTS")) > 0 {
		// To use digests the image needs to be pushed to a registry first.
		mg.Deps(
			mg.F(Build.PushImage, "package-operator-manager"),
			mg.F(Build.PushImage, remotePhasePackageName),
		)
		packageOperatorManagerImage = locations.ImageURL("package-operator-manager", true)
		remotePhasePackageImage = locations.ImageURL(remotePhasePackageName, true)
	} else {
		packageOperatorManagerImage = locations.ImageURL("package-operator-manager", false)
		remotePhasePackageImage = locations.ImageURL(remotePhasePackageName, false)
	}

	for i := range packageOperatorDeployment.Spec.Template.Spec.Containers {
		container := &packageOperatorDeployment.Spec.Template.Spec.Containers[i]

		switch container.Name {
		case "manager":
			container.Image = packageOperatorManagerImage

			for j := range container.Env {
				env := &container.Env[j]
				switch env.Name {
				case "PKO_IMAGE":
					env.Value = packageOperatorManagerImage
				case "PKO_REMOTE_PHASE_PACKAGE_IMAGE":
					env.Value = remotePhasePackageImage
				}
			}
		}
	}

	return packageOperatorDeployment
}

func patchRemotePhaseManager(scheme *k8sruntime.Scheme, obj *unstructured.Unstructured) (deploy *appsv1.Deployment) {
	// Replace image
	remotePhaseDeployment := &appsv1.Deployment{}
	if err := scheme.Convert(
		obj, remotePhaseDeployment, nil); err != nil {
		panic(fmt.Errorf("converting to Deployment: %w", err))
	}

	var (
		remotePhaseManagerImage string
	)
	if len(os.Getenv("USE_DIGESTS")) > 0 {
		// To use digests the image needs to be pushed to a registry first.
		mg.Deps(mg.F(Build.PushImage, "remote-phase-manager"))
		remotePhaseManagerImage = locations.ImageURL("remote-phase-manager", true)
	} else {
		remotePhaseManagerImage = locations.ImageURL("remote-phase-manager", false)
	}

	for i := range remotePhaseDeployment.Spec.Template.Spec.Containers {
		container := &remotePhaseDeployment.Spec.Template.Spec.Containers[i]

		switch container.Name {
		case "manager":
			container.Image = remotePhaseManagerImage
		}
	}

	return remotePhaseDeployment
}

func (l Locations) Cache() string                  { return l.cache }
func (l Locations) APISubmodule() string           { return "apis" }
func (l Locations) ClusterDeploymentCache() string { return filepath.Join(l.Cache(), "deploy") }
func (l Locations) unitTestCache() string          { return filepath.Join(l.Cache(), "unit") }
func (l Locations) UnitTestCoverageReport() string {
	return filepath.Join(l.unitTestCache(), "cov.out")
}
func (l Locations) UnitTestExecReport() string  { return filepath.Join(l.unitTestCache(), "exec.json") }
func (l Locations) IntegrationTestLogs() string { return filepath.Join(l.Cache(), "dev-env-logs") }
func (l Locations) ImageCache(imageName string) string {
	return filepath.Join(l.Cache(), "image", imageName)
}
func (l Locations) DigestFile(imgName string) string {
	return filepath.Join(l.ImageCache(imgName), imgName+".digest")
}
func (l Locations) APIReference() string    { return filepath.Join("docs", "api-reference.md") }
func (l Locations) NativeCliBinary() string { return l.binaryDst(cliCmdName, nativeArch) }
func (l Locations) Deps() magedeps.DependencyDirectory {
	return magedeps.DependencyDirectory(filepath.Join(l.Cache(), "deps"))
}
func (l Locations) binaryDst(name string, arch archTarget) string {
	if arch == nativeArch {
		return filepath.Join("bin", name)
	}

	if len(arch.OS) == 0 || len(arch.Arch) == 0 {
		panic("invalid os or arch")
	}

	return filepath.Join("bin", arch.OS+"_"+arch.Arch, name)
}

func (l Locations) ImageURL(name string, useDigest bool) string {
	envvar := strings.ReplaceAll(strings.ToUpper(name), "-", "_") + "_IMAGE"
	if url := os.Getenv(envvar); len(url) != 0 {
		return url
	}
	image := l.imageOrg + "/" + name + ":" + applicationVersion
	if !useDigest {
		return image
	}

	digest, err := os.ReadFile(locations.DigestFile(name))
	if err != nil {
		panic(err)
	}

	return l.imageOrg + "/" + name + "@" + string(digest)
}

func (l *Locations) ContainerRuntime() string {
	l.lock.Lock()
	defer l.lock.Unlock()

	if len(l.containerRuntime) == 0 {
		l.containerRuntime = os.Getenv("CONTAINER_RUNTIME")
		if len(l.containerRuntime) == 0 || l.containerRuntime == "auto" {
			cr, err := dev.DetectContainerRuntime()
			if err != nil {
				panic(err)
			}
			l.containerRuntime = string(cr)
			logger.Info("detected container-runtime", "container-runtime", l.containerRuntime)
		}
	}

	return l.containerRuntime
}

func (l *Locations) DevEnv() *dev.Environment {
	containerRuntime := l.ContainerRuntime()
	l.lock.Lock()
	defer l.lock.Unlock()

	if l.devEnvironment == nil {
		l.devEnvironment = dev.NewEnvironment(
			clusterName,
			filepath.Join(l.Cache(), "dev-env"),
			dev.WithClusterOptions([]dev.ClusterOption{
				dev.WithWaitOptions([]dev.WaitOption{dev.WithTimeout(2 * time.Minute)}),
			}),
			dev.WithContainerRuntime(containerRuntime),
			dev.WithClusterInitializers{
				dev.ClusterLoadObjectsFromFiles{
					"hack/local/hostedclusters.crd.yaml",
				},
			},
		)
	}

	return l.devEnvironment
}

// DevEnvNoInit returns the dev environment if DevelopmentEnvironment was
// already called, nil if not. This is used in case the env is optional.
func (l *Locations) DevEnvNoInit() *dev.Environment {
	l.lock.Lock()
	defer l.lock.Unlock()

	return l.devEnvironment
}

// Everything below this lines builds something. Everything above it configures said builds.
// -------------------

// Runs linters.
func (Test) Lint() { mg.SerialDeps(Test.GolangCILint, Test.GoFmt, Test.GoModTidy) }

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
	testCmd := fmt.Sprintf("go test -coverprofile=%s -race", locations.UnitTestCoverageReport())
	if isCI {
		// test output in json format
		testCmd += " -json"
	}
	testCmd += " ./internal/... ./cmd/..."

	if isCI {
		testCmd = testCmd + " > " + locations.UnitTestExecReport()
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
	os.Setenv("PKO_TEST_SUCCESS_PACKAGE_IMAGE", locations.ImageURL("test-stub-package", false))
	os.Setenv("PKO_TEST_STUB_IMAGE", locations.ImageURL("test-stub", false))

	// count=1 will force a new run, instead of using the cache
	args := []string{"test", "-v", "-failfast", "-count=1", "-timeout=20m"}
	if len(filter) > 0 {
		args = append(args, "-run", filter)
	}
	args = append(args, "./integration/...")
	testErr := sh.Run("go", args...)

	devEnv := locations.DevEnvNoInit()

	// always export logs
	if devEnv != nil {
		args := []string{"export", "logs", locations.IntegrationTestLogs(), "--name", clusterName}
		if err := devEnv.RunKindCommand(ctx, os.Stdout, os.Stderr, args...); err != nil {
			logger.Error(err, "exporting logs")
		}
	}

	if testErr != nil {
		panic(testErr)
	}
}

// Build all PKO binaries for the architecture of this machine.
func (Build) Binaries() {
	targets := []interface{}{mg.F(Build.Cmd, "mage", "", "")}
	for name := range commands {
		targets = append(targets, mg.F(Build.Cmd, name, nativeArch.OS, nativeArch.Arch))
	}

	mg.Deps(targets...)
}

func (Build) ReleaseBinaries() {
	targets := []interface{}{}
	for name, cmd := range commands {
		for _, arch := range cmd.ReleaseArchitectures {
			targets = append(targets, mg.F(Build.Cmd, name, arch.OS, arch.Arch))
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

func (Build) Binary(cmd string) { mg.Deps(mg.F(Build.Cmd, cmd, runtime.GOOS, runtime.GOARCH)) }

// Builds all PKO container images.
func (Build) Images() {
	deps := []interface{}{}
	for k := range commandImages {
		deps = append(deps, mg.F(Build.Image, k))
	}
	for k := range packageImages {
		deps = append(deps, mg.F(Build.Image, k))
	}
	mg.Deps(deps...)
}

// Builds and pushes only the given container image to the default registry.
func (Build) PushImage(imageName string) {
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

	mg.SerialDeps(mg.F(Build.Image, imageName))

	containerRuntime := locations.ContainerRuntime()

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
		args = append(args, "--digestfile="+locations.DigestFile(imageName))
	}
	args = append(args, locations.ImageURL(imageName, false))

	if err := sh.Run(containerRuntime, args...); err != nil {
		panic(fmt.Errorf("pushing image: %w", err))
	}
}

// Builds and pushes all container images to the default registry.
func (Build) PushImages() {
	deps := []interface{}{Generate.SelfBootstrapJob}
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

// Builds binaries from /cmd directory.
func (Build) Cmd(cmd, goos, goarch string) {
	env := map[string]string{"CGO_ENABLED": "0"}

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

// Builds the given container image, building binaries as prerequisite as required.
func (b Build) Image(name string) {
	_, isPkg := packageImages[name]
	_, isCmd := commandImages[name]
	switch {
	case isPkg && isCmd:
		panic("ambiguous image name")
	case isPkg:
		b.buildPackageImage(name)
	case isCmd:
		b.buildCmdImage(name)
	default:
		panic(fmt.Sprintf("unknown image: %s", name))
	}
}

// clean/prepare cache directory
func (Build) CleanImageCacheDir(name string) {
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

// generic image build function, when the image just relies on
// a static binary build from cmd/*
func (b Build) buildCmdImage(imageName string) {
	opts, ok := commandImages[imageName]
	if !ok {
		panic(fmt.Sprintf("unknown cmd image: %s", imageName))
	}
	cmd := imageName
	if len(opts.BinaryName) != 0 {
		cmd = opts.BinaryName
	}

	mg.Deps(mg.F(Build.Cmd, cmd, linuxAMD64Arch.OS, linuxAMD64Arch.Arch), mg.F(Build.CleanImageCacheDir, imageName))

	imageCacheDir := locations.ImageCache(imageName)
	imageTag := locations.ImageURL(imageName, false)

	// prepare build context
	must(sh.Copy(filepath.Join(imageCacheDir, cmd), locations.binaryDst(cmd, linuxAMD64Arch)))
	must(sh.Copy(filepath.Join(imageCacheDir, "Containerfile"), filepath.Join("config", "images", imageName+".Containerfile")))
	must(sh.Copy(filepath.Join(imageCacheDir, "passwd"), filepath.Join("config", "images", "passwd")))

	containerRuntime := locations.ContainerRuntime()
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

func (b Build) buildPackageImage(name string) {
	opts, ok := packageImages[name]
	if !ok {
		panic(fmt.Sprintf("unknown package: %s", name))
	}

	predeps := []interface{}{
		mg.F(Build.Cmd, cliCmdName, linuxAMD64Arch.OS, linuxAMD64Arch.Arch),
		mg.F(Build.CleanImageCacheDir, name),
	}
	for _, d := range opts.ExtraDeps {
		predeps = append(predeps, d)
	}
	mg.Deps(predeps...)

	imageCacheDir := locations.ImageCache(name)
	imageTag := locations.ImageURL(name, false)

	// Copy files for build environment
	must(sh.Copy(filepath.Join(imageCacheDir, "Containerfile"), filepath.Join("config", "images", "package.Containerfile")))
	must(sh.Run("cp", "-a", opts.SourcePath+"/.", imageCacheDir+"/"))

	containerRuntime := locations.ContainerRuntime()
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

// Installs all project dependencies into the local checkout.
func (d Dependency) All() {
	mg.Deps(Dependency.ControllerGen, Dependency.GolangciLint, Dependency.Kind, Dependency.Docgen)
}

// Ensure controller-gen - kubebuilder code and manifest generator.
func (d Dependency) ControllerGen() error {
	url := "sigs.k8s.io/controller-tools/cmd/controller-gen"
	return locations.Deps().GoInstall("controller-gen", url, controllerGenVersion)
}

func (d Dependency) GolangciLint() error {
	url := "github.com/golangci/golangci-lint/cmd/golangci-lint"
	return locations.Deps().GoInstall("golangci-lint", url, golangciLintVersion)
}

func (d Dependency) Docgen() error {
	url := "github.com/thetechnick/k8s-docgen"
	return locations.Deps().GoInstall("k8s-docgen", url, k8sDocGenVersion)
}

// Ensure Kind dependency - Kubernetes in Docker (or Podman)
func (d Dependency) Kind() error {
	url := "sigs.k8s.io/kind"
	return locations.Deps().GoInstall("kind", url, kindVersion)
}

// Creates an empty development environment via kind.
func (d Dev) Setup(ctx context.Context) {
	mg.SerialDeps(Dev.init)

	if err := locations.DevEnv().Init(ctx); err != nil {
		panic(fmt.Errorf("initializing dev environment: %w", err))
	}
}

// Tears the whole kind development environment down.
func (d Dev) Teardown(ctx context.Context) {
	mg.SerialDeps(Dev.init)

	if err := locations.DevEnv().Destroy(ctx); err != nil {
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
		remotePhasePackageName, pkoPackageName,
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
		fmt.Println(locations.ImageURL(images[i], false))
	}
	fmt.Println("----------------------------")
}

// Setup local cluster and deploy the Package Operator.
func (d Dev) Deploy(ctx context.Context) {
	mg.SerialDeps(Dev.Load)

	cluster := locations.DevEnv().Cluster

	d.deployPackageOperatorManager(ctx, cluster)
	d.deployPackageOperatorWebhook(ctx, cluster)
	d.deployTargetKubeConfig(ctx, cluster)
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
		packageOperatorWebhookImage = locations.ImageURL("package-operator-webhook", false)
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

func (d Dev) deployTargetKubeConfig(ctx context.Context, cluster *dev.Cluster) {
	ctx = logr.NewContext(ctx, logger)

	var err error
	// Get Kubeconfig, will be edited for the target service account
	targetKubeconfigPath := os.Getenv("TARGET_KUBECONFIG_PATH")
	var kubeconfigBytes []byte
	if len(targetKubeconfigPath) == 0 {
		kubeconfigBuf := new(bytes.Buffer)
		args := []string{"get", "kubeconfig", "--name", clusterName, "--internal"}
		err = locations.DevEnv().RunKindCommand(ctx, kubeconfigBuf, os.Stderr, args...)
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

	// Create a new secret for the kubeconfig
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "admin-kubeconfig",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"kubeconfig": kubeconfigBytes,
		},
	}

	// Deploy the secret with the new kubeconfig
	_ = cluster.CtrlClient.Delete(ctx, secret)
	if err := cluster.CtrlClient.Create(ctx, secret); err != nil {
		panic(fmt.Errorf("deploy kubeconfig secret: %w", err))
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
		packageOperatorWebhookImage = locations.ImageURL("remote-phase-manager", false)
	}
	for i := range remotePhaseManagerDeployment.Spec.Template.Spec.Containers {
		container := &remotePhaseManagerDeployment.Spec.Template.Spec.Containers[i]

		switch container.Name {
		case "manager":
			container.Image = packageOperatorWebhookImage
		}
	}

	d.deployTargetKubeConfig(ctx, cluster)

	// Beware: CreateAndWaitFromFolders doesn't update anything
	// Create the service accounts and related dependencies
	err = cluster.CreateAndWaitFromFolders(ctx, []string{filepath.Join("config", "remote-phase-static-deployment")})
	if err != nil {
		panic(fmt.Errorf("deploy remote-phase-manager dependencies: %w", err))
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

	os.Setenv("KUBECONFIG", locations.DevEnv().Cluster.Kubeconfig())

	mg.SerialDeps(Test.Integration)
}

func (d Dev) LoadImage(ctx context.Context, image string) {
	mg.Deps(mg.F(Build.Image, image))

	imageTar := filepath.Join(locations.ImageCache(image) + ".tar")
	if err := locations.DevEnv().LoadImageFromTar(ctx, imageTar); err != nil {
		panic(fmt.Errorf("load image from tar: %w", err))
	}
}

func (d Dev) init() { mg.SerialDeps(Dependency.Kind) }

// Run all code generators.
// installYamlFile has to come after code generation
func (Generate) All() { mg.SerialDeps(Generate.code, Generate.docs, Generate.installYamlFile) }

func (Generate) code() {
	mg.Deps(Dependency.ControllerGen)

	args := []string{"crd:crdVersions=v1,generateEmbeddedObjectMeta=true", "paths=./core/...", "output:crd:artifacts:config=../config/crds"}
	manifestsCmd := exec.Command("controller-gen", args...)
	manifestsCmd.Dir = locations.APISubmodule()
	manifestsCmd.Stdout = os.Stdout
	manifestsCmd.Stderr = os.Stderr
	if err := manifestsCmd.Run(); err != nil {
		panic(fmt.Errorf("generating kubernetes manifests: %w", err))
	}

	// code gen
	codeCmd := exec.Command("controller-gen", "object", "paths=./...")
	codeCmd.Dir = locations.APISubmodule()
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
	testCmd := fmt.Sprintf("go test -coverprofile=%s -race", locations.UnitTestCoverageReport())
	if isCI {
		// test output in json format
		testCmd += " -json"
	}
	testCmd += " ./internal/... ./cmd/..."

	if isCI {
		testCmd = testCmd + " > " + locations.UnitTestExecReport()
	}

	// cgo needed to enable race detector -race
	must(sh.RunWithV(map[string]string{"CGO_ENABLED": "1"}, "bash", "-c", testCmd))

	refPath := locations.APIReference()
	// Move the hack script in here.
	must(sh.RunV("bash", "-c", fmt.Sprintf("k8s-docgen apis/core/v1alpha1 > %s", refPath)))
	must(sh.RunV("bash", "-c", fmt.Sprintf("echo >> %s", refPath)))
	must(sh.RunV("bash", "-c", fmt.Sprintf("k8s-docgen apis/manifests/v1alpha1 >> %s", refPath)))
	must(sh.RunV("bash", "-c", fmt.Sprintf("echo >> %s", refPath)))
}

func (Generate) installYamlFile() {
	dumpManifestsFromFolder(filepath.Join("config", "static-deployment"), "install.yaml")
}

// Includes all static-deployment files in the package-operator-package.
func (Generate) PackageOperatorPackage() error {
	return filepath.WalkDir("config/static-deployment", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		includeInPackageOperatorPackage(path, filepath.Join("config", "packages", "package-operator"))
		return nil
	})
}

// Includes all static-deployment files in the remote-phase-package.
func (Generate) RemotePhasePackage() error {
	includeInPackageOperatorPackage(
		"config/remote-phase-static-deployment/deployment.yaml.tpl",
		filepath.Join("config", "packages", "remote-phase"))
	return nil
}

// generates a self-bootstrap-job.yaml based on the current VERSION.
// requires the images to have been build beforehand.
func (Generate) SelfBootstrapJob() {
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
		mg.Deps(mg.F(Build.PushImage, "package-operator-manager"), mg.F(Build.PushImage, pkoPackageName))
		packageOperatorManagerImage = locations.ImageURL("package-operator-manager", true)
		packageOperatorPackageImage = locations.ImageURL(pkoPackageName, true)
	} else {
		packageOperatorManagerImage = locations.ImageURL("package-operator-manager", false)
		packageOperatorPackageImage = locations.ImageURL(pkoPackageName, false)
	}

	latestJob = bytes.ReplaceAll(latestJob, []byte(pkoDefaultManagerImage), []byte(packageOperatorManagerImage))
	latestJob = bytes.ReplaceAll(latestJob, []byte(pkoDefaultPackageImage), []byte(packageOperatorPackageImage))

	must(os.WriteFile("config/self-bootstrap-job.yaml", latestJob, os.ModePerm))
}
