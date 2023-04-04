package components

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/package-operator/internal/controllers/objecttemplate"
	"package-operator.run/package-operator/internal/dynamiccache"
)

// Type alias for dependency injector to differentiate
// Cluster and non-cluster scoped *Generic<>Controllers.
type (
	ObjectTemplateController        struct{ controller }
	ClusterObjectTemplateController struct{ controller }
)

func ProvideObjectTemplateController(
	mgr ctrl.Manager, log logr.Logger,
	uncachedClient UncachedClient,
	dc *dynamiccache.Cache,
) ObjectTemplateController {
	return ObjectTemplateController{
		objecttemplate.NewObjectTemplateController(
			mgr.GetClient(), uncachedClient,
			log.WithName("controllers").WithName("ObjectTemplate"),
			dc, mgr.GetScheme(), mgr.GetRESTMapper(),
		),
	}
}

func ProvideClusterObjectTemplateController(
	mgr ctrl.Manager, log logr.Logger,
	uncachedClient UncachedClient,
	dc *dynamiccache.Cache,
) ClusterObjectTemplateController {
	return ClusterObjectTemplateController{
		objecttemplate.NewClusterObjectTemplateController(
			mgr.GetClient(), uncachedClient,
			log.WithName("controllers").WithName("ClusterObjectTemplate"),
			dc, mgr.GetScheme(), mgr.GetRESTMapper(),
		),
	}
}
