package components

import (
	"strings"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/internal/controllers/packages"
	"package-operator.run/internal/metrics"
	"package-operator.run/internal/packages/packageimport"

	pkocore "package-operator.run/apis/core/v1alpha1"
)

// Type alias for dependency injector to differentiate
// Cluster and non-cluster scoped *Generic<>Controllers.
type (
	PackageController struct {
		controllerAndEnvSinker
	}
	ClusterPackageController struct {
		controllerAndEnvSinker
	}
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
	recorder *metrics.Recorder,
	opts Options,
) PackageController {
	return PackageController{
		packages.NewPackageController[pkocore.Package, pkocore.ObjectDeployment](
			mgr.GetClient(),
			log.WithName("controllers").WithName("Package"),
			mgr.GetScheme(),
			registry, recorder, opts.PackageHashModifier,
		),
	}
}

func ProvideClusterPackageController(
	mgr ctrl.Manager, log logr.Logger,
	registry *packageimport.Registry,
	recorder *metrics.Recorder,
	opts Options,
) ClusterPackageController {
	return ClusterPackageController{
		packages.NewPackageController[pkocore.ClusterPackage, pkocore.ClusterObjectDeployment](
			mgr.GetClient(),
			log.WithName("controllers").WithName("ClusterPackage"),
			mgr.GetScheme(),
			registry, recorder, opts.PackageHashModifier,
		),
	}
}
