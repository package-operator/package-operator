package components

import (
	"github.com/go-logr/logr"
	"k8s.io/client-go/discovery"
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
	discoveryClient discovery.DiscoveryInterface,
) ObjectSetPhaseController {
	return ObjectSetPhaseController{
		objectsetphases.NewSameClusterObjectSetPhaseController(
			log.WithName("controllers").WithName("ObjectSetPhase"),
			mgr.GetScheme(), dc, uncachedClient,
			defaultObjectSetPhaseClass, mgr.GetClient(),
			mgr.GetRESTMapper(), discoveryClient,
		),
	}
}

func ProvideClusterObjectSetPhaseController(
	mgr ctrl.Manager, log logr.Logger,
	dc *dynamiccache.Cache,
	uncachedClient UncachedClient,
	discoveryClient discovery.DiscoveryInterface,
) ClusterObjectSetPhaseController {
	return ClusterObjectSetPhaseController{
		objectsetphases.NewSameClusterClusterObjectSetPhaseController(
			log.WithName("controllers").WithName("ClusterObjectSetPhase"),
			mgr.GetScheme(), dc, uncachedClient,
			defaultObjectSetPhaseClass, mgr.GetClient(),
			mgr.GetRESTMapper(), discoveryClient,
		),
	}
}
