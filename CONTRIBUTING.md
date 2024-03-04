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

## Development

### Installing `pre-commit` hooks

First install `pre-commit` as described in this [guide](https://pre-commit.com/#install).
Once `pre-commit` is installed run `pre-commit install` in the root of your clone of this
repository to enable the configured hooks. Run `pre-commit uninstall` to later disable the
hooks if needed. When installed the `pre-commit` hooks will be run on every `git commit`
action to proactively identify issues which may later cause CI to fail saving both you
and your reviewers time.

### Commands and local development

> **Dev Note**\
> Before running build targets run `export CONTAINER_RUNTIME=docker`, `export CONTAINER_RUNTIME=podman` if using `podman`, or you may get cryptic errors that may lead you to think there is a problem with the kind cluster.

Package Operator uses [Cardboard](https://github.com/package-operator/cardboard) (Think make, but all targets are written in Go instead of Shell) as task manager and developer command interface.

| Command                | Description                                                                               |
| ---------------------- | ----------------------------------------------------------------------------------------- |
| `./do Dev:Destroy`     | Deletes the local development cluster.                                                    |
| `./do Dev:Generate`    | Generate code, api docs, install files.                                                   |
| `./do Dev:Integration` | Runs local integration tests in a KinD cluster.                                           |
| `./do Dev:Lint`        | Runs local linters to check the codebase.                                                 |
| `./do Dev:LintFix`     | Tries to fix linter issues.                                                               |
| `./do Dev:Unit`        | Runs local unittests.                                                                     |


#### Accessing a cluster deployed using dev:deploy

```sh
export KUBECONFIG=$PWD/.cache/dev-env/kubeconfig.yaml
```

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
