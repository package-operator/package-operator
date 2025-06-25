package objecttemplate

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"pkg.package-operator.run/boxcutter/managedcache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"package-operator.run/internal/adapters"
	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/environment"
	"package-operator.run/internal/preflight"
)

// type dynamicCache interface {
// 	client.Reader
// 	Source(handler handler.EventHandler, predicates ...predicate.Predicate) source.Source
// 	Free(ctx context.Context, obj client.Object) error
// 	Watch(ctx context.Context, owner client.Object, obj runtime.Object) error
// 	OwnersForGKV(gvk schema.GroupVersionKind) []dynamiccache.OwnerReference
// }

type reconciler interface {
	Reconcile(ctx context.Context, pkg adapters.ObjectTemplateAccessor) (ctrl.Result, error)
}

type preflightChecker interface {
	Check(
		ctx context.Context, owner, obj client.Object,
	) (violations []preflight.Violation, err error)
}

var _ environment.Sinker = (*GenericObjectTemplateController)(nil)

type GenericObjectTemplateController struct {
	newObjectTemplate  adapters.GenericObjectTemplateFactory
	log                logr.Logger
	scheme             *runtime.Scheme
	client             client.Client
	uncachedClient     client.Client
	accessManager      managedcache.ObjectBoundAccessManager[client.Object]
	templateReconciler *templateReconciler
	reconciler         []reconciler
}

// ControllerConfig holds the configuration for the ObjectTemplate controller.
type ControllerConfig struct {
	// OptionalResourceRetryInterval is the interval at which the controller will retry
	// to fetch optional resources.
	OptionalResourceRetryInterval time.Duration
	// ResourceRetryInterval is the interval at which the controller will retry to fetch
	// resources(non optional).
	ResourceRetryInterval time.Duration
}

func NewObjectTemplateController(
	client, uncachedClient client.Client,
	log logr.Logger,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	scheme *runtime.Scheme,
	restMapper meta.RESTMapper,
	cfg ControllerConfig,
) *GenericObjectTemplateController {
	return newGenericObjectTemplateController(
		client, uncachedClient, log, accessManager, scheme,
		restMapper, adapters.NewGenericObjectTemplate, cfg)
}

func NewClusterObjectTemplateController(
	client, uncachedClient client.Client,
	log logr.Logger,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	scheme *runtime.Scheme,
	restMapper meta.RESTMapper,
	cfg ControllerConfig,
) *GenericObjectTemplateController {
	return newGenericObjectTemplateController(
		client, uncachedClient, log, accessManager, scheme,
		restMapper, adapters.NewGenericClusterObjectTemplate, cfg)
}

func newGenericObjectTemplateController(
	client, uncachedClient client.Client,
	log logr.Logger,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	scheme *runtime.Scheme,
	restMapper meta.RESTMapper,
	newObjectTemplate adapters.GenericObjectTemplateFactory,
	cfg ControllerConfig,
) *GenericObjectTemplateController {
	controller := &GenericObjectTemplateController{
		newObjectTemplate: newObjectTemplate,
		log:               log,
		scheme:            scheme,
		client:            client,
		uncachedClient:    uncachedClient,
		accessManager:     accessManager,
		templateReconciler: newTemplateReconciler(scheme, client, uncachedClient, accessManager,
			preflight.NewAPIExistence(
				restMapper,
				preflight.List{
					preflight.NewNoOwnerReferences(restMapper),
					preflight.NewEmptyNamespaceNoDefault(restMapper),
					preflight.NewNamespaceEscalation(restMapper),
				},
			),
			cfg.OptionalResourceRetryInterval,
			cfg.ResourceRetryInterval,
		),
	}
	controller.reconciler = []reconciler{controller.templateReconciler}
	return controller
}

func (c *GenericObjectTemplateController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("ObjectTemplate", req.String())
	defer log.Info("reconciled")
	ctx = logr.NewContext(ctx, log)

	objectTemplate := c.newObjectTemplate(c.scheme)
	if err := c.client.Get(
		ctx, req.NamespacedName, objectTemplate.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !objectTemplate.ClientObject().GetDeletionTimestamp().IsZero() {
		if err := c.accessManager.Free(ctx, objectTemplate.ClientObject()); err != nil {
			return ctrl.Result{}, fmt.Errorf("free cache: %w", err)
		}

		if err := controllers.RemoveCacheFinalizer(
			ctx, c.client, objectTemplate.ClientObject()); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := controllers.EnsureCachedFinalizer(ctx, c.client, objectTemplate.ClientObject()); err != nil {
		return ctrl.Result{}, err
	}

	var (
		res ctrl.Result
		err error
	)
	for _, r := range c.reconciler {
		res, err = r.Reconcile(ctx, objectTemplate)
		if err != nil || !res.IsZero() {
			break
		}
	}
	if err != nil {
		return res, err
	}
	return res, c.updateStatus(ctx, objectTemplate)
}

func (c *GenericObjectTemplateController) updateStatus(
	ctx context.Context, objectTemplate adapters.ObjectTemplateAccessor,
) error {
	if err := c.client.Status().Update(ctx, objectTemplate.ClientObject()); err != nil {
		return fmt.Errorf("updating ObjectTemplate status: %w", err)
	}
	return nil
}

func (c *GenericObjectTemplateController) SetEnvironment(env *manifests.PackageEnvironment) {
	c.templateReconciler.SetEnvironment(env)
}

func (c *GenericObjectTemplateController) SetupWithManager(
	mgr ctrl.Manager,
) error {
	objectTemplate := c.newObjectTemplate(c.scheme).ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(objectTemplate).
		WatchesRawSource(
			c.accessManager.Source(
				managedcache.NewEnqueueWatchingObjects(c.accessManager, objectTemplate, mgr.GetScheme()),
				predicate.NewPredicateFuncs(func(object client.Object) bool {
					c.log.V(constants.LogLevelDebug).Info(
						"processing dynamic cache event",
						"gvk", object.GetObjectKind().GroupVersionKind(),
						"object", client.ObjectKeyFromObject(object),
						"owners", object.GetOwnerReferences(),
					)
					return true
				}),
			),
		).
		Complete(c)
}
