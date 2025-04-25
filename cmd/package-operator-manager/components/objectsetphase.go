package components

import (
	"github.com/go-logr/logr"
	"pkg.package-operator.run/boxcutter/managedcache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/controllers/objectsetphases"
)

// Type alias for dependency injector to differentiate
// Cluster and non-cluster scoped *Generic<>Controllers.
type (
	ObjectSetPhaseController        struct{ controller }
	ClusterObjectSetPhaseController struct{ controller }
)

const defaultObjectSetPhaseClass = "default"

func ProvideObjectSetPhaseController(
	mgr ctrl.Manager, log logr.Logger,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	uncachedClient UncachedClient,
) ObjectSetPhaseController {
	return ObjectSetPhaseController{
		objectsetphases.NewSameClusterObjectSetPhaseController(
			log.WithName("controllers").WithName("ObjectSetPhase"),
			mgr.GetScheme(), accessManager, uncachedClient,
			defaultObjectSetPhaseClass, mgr.GetClient(),
			mgr.GetRESTMapper(),
		),
	}
}

func ProvideClusterObjectSetPhaseController(
	mgr ctrl.Manager, log logr.Logger,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	uncachedClient UncachedClient,
) ClusterObjectSetPhaseController {
	return ClusterObjectSetPhaseController{
		objectsetphases.NewSameClusterClusterObjectSetPhaseController(
			log.WithName("controllers").WithName("ClusterObjectSetPhase"),
			mgr.GetScheme(), accessManager, uncachedClient,
			defaultObjectSetPhaseClass, mgr.GetClient(),
			mgr.GetRESTMapper(),
		),
	}
}
