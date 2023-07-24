package components

import (
	"github.com/go-logr/logr"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/package-operator/internal/controllers/objectsets"
	"package-operator.run/package-operator/internal/dynamiccache"
	"package-operator.run/package-operator/internal/metrics"
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
	discoverClient discovery.DiscoveryInterface,
	recorder *metrics.Recorder,
) ObjectSetController {
	return ObjectSetController{
		objectsets.NewObjectSetController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("ObjectSet"),
			mgr.GetScheme(), dc, uncachedClient, discoverClient, recorder,
			mgr.GetRESTMapper(),
		),
	}
}

func ProvideClusterObjectSetController(
	mgr ctrl.Manager, log logr.Logger,
	dc *dynamiccache.Cache,
	uncachedClient UncachedClient, discoveryClient discovery.DiscoveryInterface,
	recorder *metrics.Recorder,
) ClusterObjectSetController {
	return ClusterObjectSetController{
		objectsets.NewClusterObjectSetController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("ObjectSet"),
			mgr.GetScheme(), dc, uncachedClient, discoveryClient, recorder,
			mgr.GetRESTMapper(),
		),
	}
}
