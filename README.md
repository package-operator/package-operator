# Addon Operator

<p align="center">
	<img src="docs/logo/addon-operator-github.png" width=400px>
</p>

<p align="center">
	<img src="https://prow.ci.openshift.org/badge.svg?jobs=pull-ci-openshift-addon-operator-main*">
	<img src="https://img.shields.io/github/license/openshift/addon-operator"/>
	<img src="https://img.shields.io/badge/Coolness%20Factor-Over%209000!-blue"/>
</p>

---

Addon Operator coordinates the lifecycle of Addons in managed OpenShift.

---

## Development

All development tooling can be accessed via `make`, use `make help` to get an overview of all supported targets.

This development tooling is currently used on Linux amd64, please get in touch if you need help developing from another Operating system or architecture.

### Prerequisites and Dependencies

To contribute new features or test `podman` or `docker` and the `go` tool chain need to be present on the system.

Dependencies are loaded as required and are kept local to the project in the `.cache` directory and you can setup or update all dependencies via `make dependencies`

Updating dependency versions at the top of the `Makefile` will automatically update the dependencies when they are required.

If both `docker` and `podman` are present you can explicitly force the container runtime by setting `CONTAINER_RUNTIME`.

e.g.:
```
CONTAINER_RUNTIME=docker make dev-setup
```

### Committing

Before making your first commit, please consider installing [pre-commit](https://pre-commit.com/) and run `pre-commit install` in the project directory.

Pre-commit is running some basic checks for every commit and makes our code reviews easier and prevents wasting CI resources.

### Quickstart / Develop Integration tests

Just wanting to play with the operator deployed on a cluster?

```shell
# In checkout directory:
make test-setup
```

This command will:
1. Setup a cluster via kind
2. Install OLM and OpenShift Console
3. Compile your checkout
4. Build containers
5. Load them into the kind cluster (no registry needed)
6. Install the Addon Operator

This will give you a quick environment for playing with the operator.

You can also use it to develop integration tests, against a complete setup of the Addon Operator:

```shell
# edit tests

# Run all e2e tests and skip setup and teardown,
# as the operator is already installed by: make test-setup
make test-e2e-short

# repeat!
```

### Iterate fast!

To iterate fast on code changes and experiment, the operator can also run out-of-cluster. This way we don't have to rebuild images, load them into the cluster and redeploy the operator for every code change.

Prepare the environment:

```shell
make dev-setup
```

This command will:
1. Setup a cluster via kind
2. Install OLM and OpenShift Console

```shell
# just install Addon Operator CRDs
# into the cluster.
make setup-addon-operator-crds

# Make sure we run against the new kind cluster.
export KUBECONFIG=$PWD/.cache/e2e/kubeconfig

# run the operator out-of-cluster:
# Mind your `KUBECONFIG` environment variable!
make run-addon-operator-manager
```

**Warning:**
- Your code runs as `cluster-admin`, you might run into permission errors when running in-cluster.
- Code-Generators need to be re-run and CRDs re-applied via `make setup-addon-operator-crds` when code under `./apis` is changed.
