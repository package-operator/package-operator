package components

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/internal/controllers/objectsetphases"
	"package-operator.run/internal/dynamiccache"
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
	dc *dynamiccache.Cache,
	uncachedClient UncachedClient,
) ObjectSetPhaseController {
	return ObjectSetPhaseController{
		objectsetphases.NewSameClusterObjectSetPhaseController(
			log.WithName("controllers").WithName("ObjectSetPhase"),
			mgr.GetScheme(), dc, uncachedClient,
			defaultObjectSetPhaseClass, mgr.GetClient(),
			mgr.GetRESTMapper(),
		),
	}
}

func ProvideClusterObjectSetPhaseController(
	mgr ctrl.Manager, log logr.Logger,
	dc *dynamiccache.Cache,
	uncachedClient UncachedClient,
) ClusterObjectSetPhaseController {
	return ClusterObjectSetPhaseController{
		objectsetphases.NewSameClusterClusterObjectSetPhaseController(
			log.WithName("controllers").WithName("ClusterObjectSetPhase"),
			mgr.GetScheme(), dc, uncachedClient,
			defaultObjectSetPhaseClass, mgr.GetClient(),
			mgr.GetRESTMapper(),
		),
	}
}
