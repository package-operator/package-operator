package packages

import "package-operator.run/internal/packages/internal/packageimport"

var (
	// Import a RawPackage from the given folder path.
	FromFolder = packageimport.FromFolder
	// Import a RawPackage from the given FileSystem.
	FromFS = packageimport.FromFS

	// Imports a RawPackage from the given OCI image.
	FromOCI = packageimport.FromOCI
	// Imports a RawPackage from a container image registry.
	FromRegistry = packageimport.FromRegistry

	// Creates a new registry instance to de-duplicate parallel container image pulls.
	NewRegistry = packageimport.NewRegistry
)

type (
	// Registry de-duplicates multiple parallel container image pulls.
	Registry = packageimport.Registry
)
