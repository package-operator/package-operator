package packages

import (
	"package-operator.run/internal/packages/packagedeploy"
	"package-operator.run/internal/packages/packagemanifestvalidation"
)

var (
	ValidatePackageConfiguration = packagemanifestvalidation.ValidatePackageConfiguration
	AdmitPackageConfiguration    = packagemanifestvalidation.AdmitPackageConfiguration

	ValidatePackageManifest     = packagemanifestvalidation.ValidatePackageManifest
	ValidatePackageManifestLock = packagemanifestvalidation.ValidatePackageManifestLock
)

type PackageDeployer = packagedeploy.PackageDeployer

var (
	NewPackageDeployer        = packagedeploy.NewPackageDeployer
	NewClusterPackageDeployer = packagedeploy.NewClusterPackageDeployer
)
