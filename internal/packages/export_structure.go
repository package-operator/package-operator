package packages

import "package-operator.run/internal/packages/packagestructure"

var (
	// DefaultStructuralLoader instance with the scheme pre-loaded.
	DefaultStructuralLoader = packagestructure.DefaultStructuralLoader
	// Creates a new StructuralLoaderInstance.
	NewStructuralLoader = packagestructure.NewStructuralLoader
)

// StructuralLoader parses the raw package structure to produce something usable.
type StructuralLoader = packagestructure.StructuralLoader
