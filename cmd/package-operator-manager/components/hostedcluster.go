package components

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/internal/controllers/hostedclusters"
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
			opts.PackageOperatorPackageImage,
			// use the same affinity and tolerations for remote-phase and hosted-cluster
			opts.SubComponentAffinity,
			opts.SubComponentTolerations,
		),
	}
}
