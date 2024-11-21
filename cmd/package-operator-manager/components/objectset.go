package components

import (
	"github.com/go-logr/logr"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/internal/controllers/objectsets"
	"package-operator.run/internal/dynamiccache"
	"package-operator.run/internal/metrics"
)

// Type alias for dependency injector to differentiate
// Cluster and non-cluster scoped *Generic<>Controllers.
type (
	ObjectSetController        struct{ controller }
	ClusterObjectSetController struct{ controller }
)

func ProvideObjectSetController(
	mgr ctrl.Manager, log logr.Logger,
	dc *dynamiccache.Cache,
	uncachedClient UncachedClient,
	recorder *metrics.Recorder,
	discoveryClient discovery.DiscoveryInterface,
) ObjectSetController {
	return ObjectSetController{
		objectsets.NewObjectSetController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("ObjectSet"),
			mgr.GetScheme(), dc, uncachedClient, recorder,
			mgr.GetRESTMapper(), discoveryClient,
		),
	}
}

func ProvideClusterObjectSetController(
	mgr ctrl.Manager, log logr.Logger,
	dc *dynamiccache.Cache,
	uncachedClient UncachedClient,
	recorder *metrics.Recorder,
	discoveryClient discovery.DiscoveryInterface,
) ClusterObjectSetController {
	return ClusterObjectSetController{
		objectsets.NewClusterObjectSetController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("ObjectSet"),
			mgr.GetScheme(), dc, uncachedClient, recorder,
			mgr.GetRESTMapper(), discoveryClient,
		),
	}
}
