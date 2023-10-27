package packages

import "package-operator.run/internal/packages/internal/packageexport"

var (
	// Exports the package as OCI (Open Container Image).
	ToOCI = packageexport.ToOCI
	// Exports the given package to an OCI tar under the given name and tags.
	ToOCIFile = packageexport.ToOCIFile
	// Exports the given package by pushing it to an OCI registry.
	ToPushedOCI = packageexport.ToPushedOCI
)
