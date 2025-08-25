package components

import (
	"github.com/go-logr/logr"
	"pkg.package-operator.run/boxcutter/managedcache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/controllers/objecttemplate"
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
	accessManager managedcache.ObjectBoundAccessManager[client.Object], options Options,
) ObjectTemplateController {
	return ObjectTemplateController{
		objecttemplate.NewObjectTemplateController(
			mgr.GetClient(), uncachedClient,
			log.WithName("controllers").WithName("ObjectTemplate"),
			accessManager, mgr.GetScheme(), mgr.GetRESTMapper(),
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
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	options Options,
) ClusterObjectTemplateController {
	return ClusterObjectTemplateController{
		objecttemplate.NewClusterObjectTemplateController(
			mgr.GetClient(), uncachedClient,
			log.WithName("controllers").WithName("ClusterObjectTemplate"),
			accessManager, mgr.GetScheme(), mgr.GetRESTMapper(),
			objecttemplate.ControllerConfig{
				OptionalResourceRetryInterval: options.ObjectTemplateOptionalResourceRetryInterval,
				ResourceRetryInterval:         options.ObjectTemplateResourceRetryInterval,
			},
		),
	}
}
