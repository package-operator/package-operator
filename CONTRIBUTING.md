# Contributing

## DCO

By contributing to this project you agree to the [Developer Certificate of Origin (DCO)](./DCO). This document was created by the Linux Kernel community and is a simple statement that you, as a contributor, have the legal right to make the contribution. See the DCO file for details.

## Before Opening a Pull Request

Thank you for considering making a contribution to `package-operator`.
Before opening a pull request please check that the issue/feature request
you are addressing is not already being worked on in any of the currently
open [pull requests](https://github.com/package-operator/package-operator/pulls).
If it is then please feel free to contribute by testing and reviewing the changes.

Please also check for any open [issues](https://github.com/package-operator/package-operator/issues)
that your PR may close and be sure to link those issues in your PR description.

### Testing Pull Requests Locally

To test PR's locally you must first clone this repository:

`git clone git@github.com:package-operator/package-operator.git`

Then execute the following with the correct `<PULL REQUEST NUMBER>`:

> Note: The `jq` utility must be installed on your system to run the following command.

```bash
PULL_REQUEST=<PULL REQUEST NUMBER>
git fetch $(curl -s https://api.github.com/repos/package-operator/package-operator/pulls/${PULL_REQUEST} \
     | jq -r '(.head.repo.ssh_url) + " " + (.head.ref) + ":" + (.head.ref)')
```

Alternatively you can use [GitHub CLI](https://cli.github.com/) and run `gh pr checkout <PULL REQUEST NUMBER>`.

## Development

### Installing `pre-commit` hooks

First install `pre-commit` as described in this [guide](https://pre-commit.com/#install).
Once `pre-commit` is installed run `pre-commit install` in the root of your clone of this
repository to enable the configured hooks. Run `pre-commit uninstall` to later disable the
hooks if needed. When installed the `pre-commit` hooks will be run on every `git commit`
action to proactively identify issues which may later cause CI to fail saving both you
and your reviewers time.

Running `pre-commit` hooks can be bypassed for a single commit by passing `-n` to `git commit`. Don't use this lightheartedly, as we only allow code in `main` that passes all linters and other validation checks.

### Commands and local development

> **Dev Note**\
> Before running build targets run `export CARDBOARD_CONTAINER_RUNTIME=docker`, `export CARDBOARD_CONTAINER_RUNTIME=podman` if using `podman`, or you may get cryptic errors that may lead you to think there is a problem with the kind cluster.

Package Operator uses [Cardboard](https://github.com/package-operator/cardboard) (Think make, but all targets are written in Go instead of Shell) as task manager and developer command interface.

| Command                | Description                                                                                                                     |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `./do Dev:Create`      | Sets up the local development cluster.                                                                                          |
| `./do Dev:Destroy`     | Deletes the local development cluster.                                                                                          |
| `./do Dev:Generate`    | Generate code, api docs, install files.                                                                                         |
| `./do Dev:Integration` | Runs local integration tests in a KinD cluster. (Run `Dev:Destroy` before to ensure that you're testing a freshly compiled PKO) |
| `./do Dev:Lint`        | Runs local linters to check the codebase.                                                                                       |
| `./do Dev:LintFix`     | Tries to fix linter issues.                                                                                                     |
| `./do Dev:Unit`        | Runs local unittests.                                                                                                           |
| `./do Dev:Run`         | Prepares development cluster and `go runs` package-operator-manager out-of-cluster.                                             |

#### Setting up the local development cluster without shadowing image pulling (pod workload images and package-operator package images) from quay.io

By default, the local development environment will redirect image pulls from `quay.io` to the local dev registry running at `localhost:5000` instead.

These redirects affect:
- package-operator-manager: will pull package images when a `(Cluster)Package` object changes.
- kubelet/container-runtime: will pull workload images specified in `Pod` objects.

If you need to access (package) images from `quay.io` you can move the override out of the way by:
1. Ensuring to fully destroy your current development environment, by running: `./do Dev:Destroy`.
2. Recreating the development environment with another registry prefix (that will be redirected to localhost:5000, which in turn frees up access to quay.io): `IMAGE_REGISTRY=ctr.package-operator.run/dev ./do Dev:Create`.
3. Prepend all other cardboard commands with `IMAGE_REGISTRY=ctr.package-operator.run/dev` (or export it via an `.envrc`).

We're thinking of changing this away from quay.io by default.

#### Accessing a local development cluster created by cardboard

Replace `<cluster_name>` with either of:
- `pko`: for the main development cluster.
- `pko-hs-hc`: for the faux "HyperShift Hosted Cluster".

```sh
export KUBECONFIG=$PWD/.cache/clusters/<cluster_name>/kubeconfig.yaml
```

You'll mostly use the main development cluster: `export KUBECONFIG=$PWD/.cache/clusters/pko/kubeconfig.yaml`

### Running Tests

#### Linters

```sh
./do Dev:LintFix
```

#### Unit Tests

```sh
./do Dev:Unit
```

#### Integration Tests

Create a local [KinD](https://kind.sigs.k8s.io/) cluster and run integration
suite on it.

```sh
./do Dev:Integration
```

Regardless of whether the integration suite passes or fails the cluster created
must be explicitly cleaned up afterwards.

```sh
./do Dev:Destroy
```

## Submitting Pull Requests

First fork this repository and create a branch off of `main` to commit to.
Commits should be named as per
[conventional commits](https://www.conventionalcommits.org/en/v1.0.0/).
When submitting your PR fill in the pull request template to the best of
your abilities. If you are not a member of the _package-operator_ organization a
member will have to leave a comment for CI to run checks against your
PR so please be patient until a member can review.
