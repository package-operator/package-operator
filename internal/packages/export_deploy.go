package packages

import (
	"package-operator.run/internal/packages/internal/packagedeploy"
)

// PackageDeployer loads package contents from file, wraps it into an ObjectDeployment and deploys it.
type PackageDeployer = packagedeploy.PackageDeployer

var (
	// Returns a new namespace-scoped loader for the Package API.
	NewPackageDeployer = packagedeploy.NewPackageDeployer
	// Returns a new cluster-scoped loader for the ClusterPackage API.
	NewClusterPackageDeployer = packagedeploy.NewClusterPackageDeployer
)
