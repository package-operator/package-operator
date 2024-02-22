package packaging

import "package-operator.run/internal/packages"

var (
	// FromFolder imports a RawPackage from the given folder path.
	FromFolder = packages.FromFolder
	// FromFS import a RawPackage from the given FileSystem.
	FromFS = packages.FromFS
	// FromOCI imports a RawPackage from the given OCI image.
	FromOCI = packages.FromOCI
	// FromRegistry imports a RawPackage from a container image registry.
	FromRegistry = packages.FromRegistry
)
