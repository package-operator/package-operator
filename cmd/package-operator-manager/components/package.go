package components

import (
	"strings"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/package-operator/internal/controllers/packages"
	"package-operator.run/package-operator/internal/packages/packageimport"
)

// Type alias for dependency injector to differentiate
// Cluster and non-cluster scoped *Generic<>Controllers.
type (
	PackageController        struct{ controller }
	ClusterPackageController struct{ controller }
)

func ProvideRegistry(log logr.Logger, opts Options) *packageimport.Registry {
	return packageimport.NewRegistry(
		prepareRegistryHostOverrides(log, opts.RegistryHostOverrides))
}

func prepareRegistryHostOverrides(log logr.Logger, flag string) map[string]string {
	if len(flag) == 0 {
		return nil
	}

	log.WithName("Registry").Info("registry host overrides active", "overrides", flag)
	out := map[string]string{}
	overrides := strings.Split(flag, ",")
	for _, or := range overrides {
		parts := strings.SplitN(or, "=", 2)
		if len(parts) != 2 {
			continue
		}
		out[parts[0]] = parts[1]
	}
	return out
}

func ProvidePackageController(
	mgr ctrl.Manager, log logr.Logger,
	registry *packageimport.Registry,
) PackageController {
	return PackageController{
		packages.NewPackageController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("Package"),
			mgr.GetScheme(),
			registry,
		),
	}
}

func ProvideClusterPackageController(
	mgr ctrl.Manager, log logr.Logger,
	registry *packageimport.Registry,
) ClusterPackageController {
	return ClusterPackageController{
		packages.NewClusterPackageController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("ClusterPackage"),
			mgr.GetScheme(),
			registry,
		),
	}
}
