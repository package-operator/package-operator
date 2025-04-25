package components

import (
	"github.com/go-logr/logr"
	"pkg.package-operator.run/boxcutter/managedcache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/controllers/objectsets"
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
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	uncachedClient UncachedClient,
	recorder *metrics.Recorder,
) ObjectSetController {
	return ObjectSetController{
		objectsets.NewObjectSetController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("ObjectSet"),
			mgr.GetScheme(), accessManager, uncachedClient, recorder,
			mgr.GetRESTMapper(),
		),
	}
}

func ProvideClusterObjectSetController(
	mgr ctrl.Manager, log logr.Logger,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	uncachedClient UncachedClient,
	recorder *metrics.Recorder,
) ClusterObjectSetController {
	return ClusterObjectSetController{
		objectsets.NewClusterObjectSetController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("ObjectSet"),
			mgr.GetScheme(), accessManager, uncachedClient, recorder,
			mgr.GetRESTMapper(),
		),
	}
}
