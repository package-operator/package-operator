//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"path"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/mt-sre/devkube/magedeps"
)

// Directories
var (
	// Working directory of the project.
	workDir string
	// Dependency directory.
	depsDir magedeps.DependencyDirectory
)

// Testing and Linting
// -------------------

type Test mg.Namespace

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
	needsRebuild, err := depsDir.NeedsRebuild("opm", opmVersion)
	if err != nil {
		return err
	}
	if !needsRebuild {
		return nil
	}

	// Tempdir
	tempDir, err := os.MkdirTemp(".cache", "")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download
	tempOPMBin := path.Join(tempDir, "opm")
	if err := sh.Run(
		"curl", "-L", "--fail",
		"-o", tempOPMBin,
		fmt.Sprintf(
			"https://github.com/operator-framework/operator-registry/releases/download/v%s/linux-amd64-opm",
			opmVersion,
		),
	); err != nil {
		return fmt.Errorf("downloading protoc: %w", err)
	}

	if err := os.Chmod(tempOPMBin, 0755); err != nil {
		return fmt.Errorf("make opm executable: %w", err)
	}

	// Move
	if err := os.Rename(tempOPMBin, path.Join(depsDir.Bin(), "opm")); err != nil {
		return fmt.Errorf("move protoc: %w", err)
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

	depsDir = magedeps.DependencyDirectory(path.Join(workDir, ".deps"))
}
