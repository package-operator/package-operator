package components

import (
	"github.com/go-logr/logr"
	"k8s.io/client-go/discovery"
	"pkg.package-operator.run/boxcutter/machinery"
	"pkg.package-operator.run/boxcutter/managedcache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/constants"
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
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	discoveryClient discovery.DiscoveryInterface,
	options Options,
) ObjectTemplateController {
	return ObjectTemplateController{
		objecttemplate.NewObjectTemplateController(
			mgr.GetClient(), uncachedClient,
			log.WithName("controllers").WithName("ObjectTemplate"),
			accessManager, mgr.GetScheme(), mgr.GetRESTMapper(),
			machinery.NewComparator(
				discoveryClient,
				mgr.GetScheme(),
				constants.ObjectTemplateFieldOwner, // must be different than other controllers in PKO to avoid removing finalizers and similar changes added by them.
			),
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
	discoveryClient discovery.DiscoveryInterface,
	options Options,
) ClusterObjectTemplateController {
	return ClusterObjectTemplateController{
		objecttemplate.NewClusterObjectTemplateController(
			mgr.GetClient(), uncachedClient,
			log.WithName("controllers").WithName("ClusterObjectTemplate"),
			accessManager, mgr.GetScheme(), mgr.GetRESTMapper(),
			machinery.NewComparator(
				discoveryClient,
				mgr.GetScheme(),
				constants.ObjectTemplateFieldOwner, // must be different than other controllers in PKO to avoid removing finalizers and similar changes added by them.
			),
			objecttemplate.ControllerConfig{
				OptionalResourceRetryInterval: options.ObjectTemplateOptionalResourceRetryInterval,
				ResourceRetryInterval:         options.ObjectTemplateResourceRetryInterval,
			},
		),
	}
}
