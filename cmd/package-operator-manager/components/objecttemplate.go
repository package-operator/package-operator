package components

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/internal/controllers/objecttemplate"
	"package-operator.run/internal/dynamiccache"
)

// Type alias for dependency injector to differentiate
// Cluster and non-cluster scoped *Generic<>Controllers.
type (
	ObjectTemplateController struct {
		controllerAndEnvSinker
	}
	ClusterObjectTemplateController struct {
		controllerAndEnvSinker
	}
)

func ProvideObjectTemplateController(
	mgr ctrl.Manager, log logr.Logger,
	uncachedClient UncachedClient,
	dc *dynamiccache.Cache, options Options,
) ObjectTemplateController {
	return ObjectTemplateController{
		objecttemplate.NewObjectTemplateController(
			mgr.GetClient(), uncachedClient,
			log.WithName("controllers").WithName("ObjectTemplate"),
			dc, mgr.GetScheme(), mgr.GetRESTMapper(),
			objecttemplate.ControllerConfig{
				OptionalResourceRetryInterval: options.ObjectTemplateOptionalResourceRetryInterval,
				ResourceRetryInterval:         options.ObjectTemplateResourceRetryInterval,
			},
		),
	}
}

func ProvideClusterObjectTemplateController(
	mgr ctrl.Manager, log logr.Logger,
	uncachedClient UncachedClient,
	dc *dynamiccache.Cache,
	options Options,
) ClusterObjectTemplateController {
	return ClusterObjectTemplateController{
		objecttemplate.NewClusterObjectTemplateController(
			mgr.GetClient(), uncachedClient,
			log.WithName("controllers").WithName("ClusterObjectTemplate"),
			dc, mgr.GetScheme(), mgr.GetRESTMapper(),
			objecttemplate.ControllerConfig{
				OptionalResourceRetryInterval: options.ObjectTemplateOptionalResourceRetryInterval,
				ResourceRetryInterval:         options.ObjectTemplateResourceRetryInterval,
			},
		),
	}
}
