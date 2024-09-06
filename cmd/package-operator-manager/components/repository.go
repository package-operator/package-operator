package components

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/internal/packages"

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
	mgr ctrl.Manager, log logr.Logger, store packages.RepositoryStore,
) RepositoryController {
	return RepositoryController{
		controllersrepositories.NewRepositoryController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("Repository"),
			mgr.GetScheme(),
			store,
		),
	}
}

func ProvideClusterRepositoryController(
	mgr ctrl.Manager, log logr.Logger, store packages.RepositoryStore,
) ClusterRepositoryController {
	return ClusterRepositoryController{
		controllersrepositories.NewClusterRepositoryController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("ClusterRepository"),
			mgr.GetScheme(),
			store,
		),
	}
}

func ProvideRepositoryStore() packages.RepositoryStore {
	return packages.NewRepositoryStore()
}
