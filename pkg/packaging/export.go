package packaging

import "package-operator.run/internal/packages"

var (
	// ToOCIFile exports the given package to an OCI tar under the given name and tags.
	ToOCIFile = packages.ToOCIFile
	// ToPushedOCI exports the given package by pushing it to an OCI registry.
	ToPushedOCI = packages.ToPushedOCI
)
