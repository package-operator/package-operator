//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"

	"github.com/magefile/mage/mg"
)

type Dependency mg.Namespace

// Installs all project dependencies into the local checkout.
func (d Dependency) All() {
	mg.Deps(
		Dependency.ControllerGen,
		Dependency.GolangciLint,
		Dependency.Kind,
		Dependency.Docgen,
		Dependency.Crane,
		Dependency.Helm,
	)
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

func (d Dependency) Crane() error {
	url := "github.com/google/go-containerregistry/cmd/crane"
	return locations.Deps().GoInstall("crane", url, craneVersion)
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

func (d Dependency) Helm() error {
	url := "helm.sh/helm/v3/cmd/helm"
	return locations.Deps().GoInstall("helm", url, helmVersion)
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
