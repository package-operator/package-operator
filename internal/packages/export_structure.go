package packages

import "package-operator.run/internal/packages/internal/packagestructure"

var (
	// DefaultStructuralLoader instance with the scheme pre-loaded.
	DefaultStructuralLoader = packagestructure.DefaultStructuralLoader
	// Creates a new StructuralLoaderInstance.
	NewStructuralLoader = packagestructure.NewStructuralLoader
	// Converts the internal version of an PackageManifestLock into it's v1alpha1 representation.
	ToV1Alpha1ManifestLock = packagestructure.ToV1Alpha1ManifestLock
)

// StructuralLoader parses the raw package structure to produce something usable.
type StructuralLoader = packagestructure.StructuralLoader
