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

## dev tools

- setup pre-commit hooks: `make pre-commit-install`
- global requirements:
	- golang
	- kubectl/oc
	- make
	- either docker or podman
