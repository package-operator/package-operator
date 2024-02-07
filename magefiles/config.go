//go:build mage
// +build mage

package main

import (
	"os"
	"path/filepath"
	"runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// Initialize all the global variables.
func init() {
	// Use a local directory to get around permission errors in OpenShift CI.
	os.Setenv("GOLANGCI_LINT_CACHE", filepath.Join(locations.Cache(), "golangci-lint"))
	os.Setenv("PATH", locations.Deps().Bin()+":"+locations.bin+":"+os.Getenv("PATH"))

	// Extra dependencies must be specified here to avoid a circular dependency.
	packageImages[pkoPackageName].ExtraDeps = []any{Generate.PackageOperatorPackage}
	packageImages[remotePhasePackageName].ExtraDeps = []any{Generate.RemotePhasePackage}

	ctrl.SetLogger(logger)
}

// Constants that define build behaviour.
const (
	module                       = "package-operator.run"
	defaultImageOrg              = "quay.io/package-operator"
	clusterName                  = "package-operator-dev"
	cliCmdName                   = "kubectl-package"
	pkoPackageName               = "package-operator-package"
	remotePhasePackageName       = "remote-phase-package"
	defaultPKOLatestBootstrapJob = "https://github.com/package-operator/package-operator/releases/latest/download/self-bootstrap-job.yaml"

	controllerGenVersion = "0.14.0"
	conversionGenVersion = "0.29.1"
	golangciLintVersion  = "1.55.2"
	craneVersion         = "0.19.0"
	kindVersion          = "0.21.0"
	k8sDocGenVersion     = "0.6.2"
	helmVersion          = "3.14.0"
	testFMTVersion       = "2.5.0"
)

// Types for target configuration.
type (
	archTarget struct{ OS, Arch string }
	command    struct{ ReleaseArchitectures []archTarget }

	CommandImage struct {
		Push       bool
		BinaryName string
	}
	PackageImage struct {
		ExtraDeps  []any
		Push       bool
		SourcePath string
	}
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
	// since that would result in a circular dependency. They must be added via init() for now.
	packageImages = map[string]*PackageImage{
		pkoPackageName:            {Push: true, SourcePath: filepath.Join("config", "packages", "package-operator")},
		remotePhasePackageName:    {Push: true, SourcePath: filepath.Join("config", "packages", "remote-phase")},
		"test-stub-package":       {SourcePath: filepath.Join("config", "packages", "test-stub")},
		"test-stub-multi-package": {SourcePath: filepath.Join("config", "packages", "test-stub-multi")},
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
	nativeArch         = archTarget{runtime.GOOS, runtime.GOARCH}
	linuxAMD64Arch     = archTarget{"linux", "amd64"}
	locations          = newLocations()
	logger             = zap.New(zap.UseDevMode(true))
	applicationVersion string
	imageRegistry      string
	// Push to development registry instead of pushing to `imageRegistry`.
	pushToDevRegistry bool
)
