# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Package Operator is a Kubernetes operator that manages packages as collections of arbitrary objects to install and maintain applications on one or multiple clusters. It's written in Go and uses the controller-runtime framework.

## Development Commands

The project uses Cardboard (a task manager written in Go) as the primary build system interface. All commands are executed through the `./do` script:

### Essential Development Commands
- `./do Dev:Create` - Sets up the local development cluster (KinD)
- `./do Dev:Destroy` - Deletes the local development cluster
- `./do Dev:Bootstrap` - Deploys package-operator to local development cluster
- `./do Dev:Generate` - Generate code, API docs, install files
- `./do Dev:Lint` - Runs local linters to check the codebase
- `./do Dev:LintFix` - Tries to fix linter issues
- `./do Dev:Unit` - Runs local unit tests
- `./do Dev:Integration` - Runs local integration tests in a KinD cluster
- `./do Dev:Run` - Prepares development cluster and runs package-operator-manager out-of-cluster

### Important Notes
- Before running build targets, set `export CARDBOARD_CONTAINER_RUNTIME=docker` (or `podman` if using Podman)
- Run `./do Dev:Destroy` before integration tests to ensure testing a freshly compiled PKO
- Integration tests require explicit cleanup: `./do Dev:Destroy` after completion

### Pre-commit Hooks
- Install with `pre-commit install` in the repository root
- Can be bypassed for single commit with `git commit -n` (use sparingly)

## Repository Structure

### Multi-Module Go Workspace
The project uses Go workspaces with three modules:
- **Root module** (`package-operator.run`) - Main application and CLI tools
- **APIs module** (`package-operator.run/apis`) - Kubernetes API definitions
- **PKG module** (`package-operator.run/pkg`) - Shared packages and utilities

### Key Directories
- `cmd/` - Main applications:
  - `cmd/build/` - Cardboard build system implementation
  - `cmd/package-operator-manager/` - Main operator manager
  - `cmd/kubectl-package/` - kubectl plugin for package management
- `apis/` - Kubernetes API definitions (separate module)
  - `apis/core/v1alpha1/` - Core Package Operator APIs
  - `apis/manifests/v1alpha1/` - Package manifest APIs
- `pkg/` - Shared libraries (separate module)

### Core API Types
The Package Operator defines several key Kubernetes resources:
- **Package/ClusterPackage** - Main package installation objects
- **ObjectSet/ClusterObjectSet** - Groups of objects managed together
- **ObjectDeployment/ClusterObjectDeployment** - Deployment management
- **ObjectSetPhase/ClusterObjectSetPhase** - Phased rollout support
- **PackageManifest** - Package metadata and structure definitions

## Architecture Overview

### Package Management System
Package Operator implements a hierarchical package management system:
1. **Packages** contain manifests and templates for Kubernetes resources
2. **ObjectSets** group related objects and manage their lifecycle
3. **ObjectDeployments** handle deployment strategies and rollouts
4. **ObjectSetPhases** enable multi-phase installations with dependencies

### Controller Architecture
The operator follows the standard Kubernetes controller pattern:
- Controllers watch for changes to custom resources
- Reconciliation loops ensure desired state matches actual state
- Uses controller-runtime framework for reliable operation
- Supports both namespaced and cluster-scoped operations

### Local Development Environment
- Uses KinD (Kubernetes in Docker) for local testing
- Local registry at `localhost:5000` for development images
- Redirects `quay.io` pulls to local registry by default
- Supports HyperShift hosted cluster testing with `pko-hs-hc` cluster

## Development Workflow

### Setting Up Development Environment
1. Clone repository
2. Install pre-commit hooks: `pre-commit install`
3. Set container runtime: `export CARDBOARD_CONTAINER_RUNTIME=docker`
4. Create development cluster: `./do Dev:Create`
5. Bootstrap operator: `./do Dev:Bootstrap`

### Making Changes
1. Run linters: `./do Dev:LintFix`
2. Run unit tests: `./do Dev:Unit`
3. Generate code if needed: `./do Dev:Generate`
4. Test integration: `./do Dev:Integration`
5. Clean up: `./do Dev:Destroy`

### Accessing Development Clusters
```sh
# Main development cluster
export KUBECONFIG=$PWD/.cache/clusters/pko/kubeconfig.yaml

# HyperShift hosted cluster (if needed)
export KUBECONFIG=$PWD/.cache/clusters/pko-hs-hc/kubeconfig.yaml
```

### Conventional Commits
All commits must follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) standard for consistent versioning and changelog generation.

## Key Tools and Dependencies

### Go Tools (managed by Cardboard)
- `controller-gen` - Kubernetes code generation
- `conversion-gen` - API version conversion
- `golangci-lint` - Go linting
- `helm` - Helm chart management
- `crane` - Container registry operations
- `govulncheck` - Vulnerability scanning

### Testing Framework
- Uses Ginkgo/Gomega for BDD-style testing
- Integration tests run against real KinD clusters
- Unit tests for individual components and utilities

### Container and Registry Management
- Supports Docker and Podman container runtimes
- Local registry setup for development
- OCI image handling for package distribution