package components

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	controllersrepositories "package-operator.run/internal/controllers/repositories"
)

// Type alias for dependency injector to differentiate
// Cluster and non-cluster scoped *Generic<>Controllers.
type (
	RepositoryController struct {
		controller
	}
	ClusterRepositoryController struct {
		controller
	}
)

func ProvideRepositoryController(
	mgr ctrl.Manager, log logr.Logger,
) RepositoryController {
	return RepositoryController{
		controllersrepositories.NewRepositoryController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("Repository"),
			mgr.GetScheme(),
		),
	}
}

func ProvideClusterRepositoryController(
	mgr ctrl.Manager, log logr.Logger,
) ClusterRepositoryController {
	return ClusterRepositoryController{
		controllersrepositories.NewClusterRepositoryController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("ClusterRepository"),
			mgr.GetScheme(),
		),
	}
}
