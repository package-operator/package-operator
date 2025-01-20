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

	// Imports a RawPackage from a container image registry,
	// while supplying pull credentials which are dynamically discovered from the ServiceAccount PKO is running under.
	FromRegistryInCluster = packageimport.FromRegistryInCluster

	// Creates a new registry instance to de-duplicate parallel container image pulls.
	NewRequestManager = packageimport.NewRequestManager
)

type (
	// RequestManager de-duplicates multiple parallel container image pulls.
	RequestManager = packageimport.RequestManager
)
