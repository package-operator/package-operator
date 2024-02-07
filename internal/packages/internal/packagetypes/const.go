package packagetypes

const (
	// OCIPathPrefix defines under which subfolder files within a package container should be located.
	OCIPathPrefix = "package"
	// Package manifest filename without file-extension.
	PackageManifestFilename = "manifest"
	// Package manifest lock filename without file-extension.
	PackageManifestLockFilename = "manifest.lock"
	// Name of the components folder for multi-components.
	ComponentsFolder = "components"
	// Name of the test fixtures folder used for template validation.
	PackageTestFixturesFolder = ".test-fixtures"
)
