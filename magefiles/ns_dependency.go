//go:build mage
// +build mage

package main

import (
	"fmt"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

type Dependency mg.Namespace

// Installs all project dependencies into the local checkout.
func (d Dependency) All() {
	mg.Deps(
		Dependency.ControllerGen,
		Dependency.ConversionGen,
		Dependency.GolangciLint,
		Dependency.Kind,
		Dependency.Docgen,
		Dependency.Helm,
	)
}

func install(target string) {
	sh.RunV(mg.GoCmd(), "install", target)
}

// Ensure controller-gen - kubebuilder code and manifest generator.
func (d Dependency) ControllerGen() {
	install(fmt.Sprintf("sigs.k8s.io/controller-tools/cmd/controller-gen@%s", controllerGenVersion))
}

func (d Dependency) ConversionGen() {
	install(fmt.Sprintf("k8s.io/code-generator/cmd/conversion-gen@%s", conversionGenVersion))
}

func (d Dependency) GolangciLint() {
	install(fmt.Sprintf("github.com/golangci/golangci-lint/cmd/golangci-lint@%s", golangciLintVersion))
}

func (d Dependency) Docgen() {
	install(fmt.Sprintf("github.com/thetechnick/k8s-docgen@%s", k8sDocGenVersion))
}

// Ensure Kind dependency - Kubernetes in Docker (or Podman)
func (d Dependency) Kind() {
	install(fmt.Sprintf("sigs.k8s.io/kind@%s", kindVersion))
}

func (d Dependency) Helm() {
	install(fmt.Sprintf("helm.sh/helm/v3/cmd/helm@%s", helmVersion))
}
