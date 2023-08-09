package components

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/internal/controllers/objectdeployments"
)

// Type alias for dependency injector to differentiate
// Cluster and non-cluster scoped *Generic<>Controllers.
type (
	ObjectDeploymentController        struct{ controller }
	ClusterObjectDeploymentController struct{ controller }
)

func ProvideObjectDeploymentController(
	mgr ctrl.Manager, log logr.Logger,
) ObjectDeploymentController {
	return ObjectDeploymentController{
		objectdeployments.NewObjectDeploymentController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("ObjectDeployment"),
			mgr.GetScheme(),
		),
	}
}

func ProvideClusterObjectDeploymentController(
	mgr ctrl.Manager, log logr.Logger,
) ClusterObjectDeploymentController {
	return ClusterObjectDeploymentController{
		objectdeployments.NewClusterObjectDeploymentController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("ClusterObjectDeployment"),
			mgr.GetScheme(),
		),
	}
}
