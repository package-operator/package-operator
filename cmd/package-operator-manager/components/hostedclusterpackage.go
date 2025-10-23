package components

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/internal/controllers/hostedclusterpackages"
)

// Type alias for dependency injector to differentiate
// Cluster and non-cluster scoped *Generic<>Controllers.
type HostedClusterPackageController struct{ controller }

func ProvideHostedClusterPackageController(
	mgr ctrl.Manager, log logr.Logger,
) HostedClusterPackageController {
	return HostedClusterPackageController{
		hostedclusterpackages.NewHostedClusterPackageController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("HostedClusterPackage"),
			mgr.GetScheme(),
		),
	}
}
