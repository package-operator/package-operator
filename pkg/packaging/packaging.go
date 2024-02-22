package packaging

import (
	"context"

	"package-operator.run/internal/packages"
)

type (
	// Package has passed basic schema/structure admission.
	// Exact output still depends on configuration and
	// the install environment.
	Package = packages.Package
	// RawPackage right after import.
	// No validation has been performed yet.
	RawPackage = packages.RawPackage
	// Files is an in-memory representation of the package FileSystem.
	// It maps file paths to their contents.
	Files = packages.Files
)

// Load takes a raw package, as import from file or OCI and
// parses it's folder structure and manifests.
func Load(ctx context.Context, rawPkg *RawPackage) (*Package, error) {
	return packages.DefaultStructuralLoader.Load(ctx, rawPkg)
}
