package packages

import "package-operator.run/internal/packages/internal/packagetypes"

const (
	// Package manifest filename without file-extension.
	PackageManifestFilename = packagetypes.PackageManifestFilename
	// Package manifest lock filename without file-extension.
	PackageManifestLockFilename = packagetypes.PackageManifestLockFilename
	// Name of the test fixtures folder used for template validation.
	PackageTestFixturesFolder = packagetypes.PackageTestFixturesFolder
)

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

var (
	// PackageManifestGroupKind is the kubernetes schema group kind of a PackageManifest.
	PackageManifestGroupKind = packagetypes.PackageManifestGroupKind
	// PackageManifestLockGroupKind is the kubernetes schema group kind of a PackageManifestLock.
	PackageManifestLockGroupKind = packagetypes.PackageManifestLockGroupKind
)
