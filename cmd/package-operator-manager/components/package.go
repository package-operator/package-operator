package components

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/package-operator/internal/controllers/packages"
)

// Type alias for dependency injector to differentiate
// Cluster and non-cluster scoped *Generic<>Controllers.
type (
	PackageController        struct{ controller }
	ClusterPackageController struct{ controller }
)

func ProvidePackageController(
	mgr ctrl.Manager, log logr.Logger,
	opts Options,
) PackageController {
	return PackageController{
		packages.NewPackageController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("Package"),
			mgr.GetScheme(),
			opts.Namespace, opts.ManagerImage,
		),
	}
}

func ProvideClusterPackageController(
	mgr ctrl.Manager, log logr.Logger,
	opts Options,
) ClusterPackageController {
	return ClusterPackageController{
		packages.NewClusterPackageController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("ClusterPackage"),
			mgr.GetScheme(),
			opts.Namespace, opts.ManagerImage,
		),
	}
}
