package components

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/package-operator/internal/controllers/hostedclusters"
)

// Type alias for dependency injector to differentiate
// Cluster and non-cluster scoped *Generic<>Controllers.
type HostedClusterController struct{ controller }

func ProvideHostedClusterController(
	mgr ctrl.Manager, log logr.Logger,
	opts Options,
) HostedClusterController {
	return HostedClusterController{
		hostedclusters.NewHostedClusterController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("HostedCluster"),
			mgr.GetScheme(),
			opts.RemotePhasePackageImage,
		),
	}
}
