package packages

import "package-operator.run/internal/packages/packagetypes"

type (
	// Package has passed basic schema/structure admission.
	// Exact output still depends on configuration and
	// the install environment.
	Package = packagetypes.Package
	// PackageInstance is the concrete instance of a package after rendering
	// templates from configuration and environment information.
	PackageInstance = packagetypes.PackageInstance
	// PackageRenderContext contains all data that is needed to render a Package into a PackageInstance.
	PackageRenderContext = packagetypes.PackageRenderContext
	// RawPackage right after import.
	// No validation has been performed yet.
	RawPackage = packagetypes.RawPackage
	// Files is an in-memory representation of the package FileSystem.
	// It maps file paths to their contents.
	Files = packagetypes.Files
)
