# Package Operator

<p align="center">
	<img src="docs/logos/package-operator-github.png" width=400px>
</p>

<p align="center">
	<img src="https://img.shields.io/github/license/package-operator/package-operator"/>
</p>

---

Operator for packaging and managing a collection of arbitrary Kubernetes objects to install software on one or multiple clusters.

---
Dev Note: Assure you have `export CONTAINER_RUNTIME=docker` or similar for `podman`, or you will get cryptic errors from
mage that may lead you to think there is a problem with Kind cluster.
