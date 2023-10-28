# Package Operator

<p align="center">
	<img src="docs/logos/package-operator-github.png" width=400px>
</p>

<p align="center">
	<a href="https://package-operator.run">
		<img src="https://img.shields.io/badge/docs-package--operator.run-blue?style=flat-square" alt="Documentation"/>
	</a>
	<a href="https://pkg.go.dev/package-operator.run/apis">
		<img src="https://pkg.go.dev/badge/package-operator.run/apis" />
	</a>
	<img src="https://img.shields.io/github/license/package-operator/package-operator?style=flat-square"/>
	<img src="https://img.shields.io/github/go-mod/go-version/package-operator/package-operator?style=flat-square"/>
	<img src="https://img.shields.io/codecov/c/gh/package-operator/package-operator?style=flat-square"/>
</p>

---

Package Operator is an open source operator for [Kubernetes](https://kubernetes.io/), managing packages as collections of arbitrary objects, to install and maintain applications on one or multiple clusters.

---

- [Project Status](#project-status)
- [Features](#features)
- [Documentation](#documentation)
- [Getting in touch](#getting-in-touch)
- [Contributing](#contributing)
- [License](#license)

---

## Project Status

Package Operator is used in production and the concepts proven.

The Core APIs are generally stable and breaking changes should only happen in exceptional circumstances.\
Be careful to check the change notes for alpha and beta APIs.

## Features

- No Surprises
	- Ordered Installation and Removal
	- Operating Transparency
- Extensible
	- Declarative APIs
	- Plug and Play
- Cheap Failures and Easy Recovery
	- Rollout History
	- Rollback

## Documentation

Package Operator documentation is available on [package-operator.run](https://package-operator.run).

The source of this website is our [website repository](https://github.com/package-operator/package-operator.github.io) which is hosted via Github Pages, [Hugo](https://gohugo.io/) and using the [Doks template](https://getdoks.org/).

## Getting in touch

Our mailing lists:
- [pko-devel](https://groups.google.com/g/pko-devel) for development discussions.
- [pko-users](https://groups.google.com/g/pko-users) for discussions among users and potential users.

## Contributing

Thank you for taking time to help to improve Package Operator!

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for instructions on how to contribute.

## License

Package Operator is [Apache 2.0 licensed](./LICENSE).
