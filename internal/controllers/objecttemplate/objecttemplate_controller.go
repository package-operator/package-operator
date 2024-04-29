package objecttemplate

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/dynamiccache"
	"package-operator.run/internal/environment"
	"package-operator.run/internal/preflight"
)

type dynamicCache interface {
	client.Reader
	Source(handler handler.EventHandler, predicates ...predicate.Predicate) source.Source
	Free(ctx context.Context, obj client.Object) error
	Watch(ctx context.Context, owner client.Object, obj runtime.Object) error
	OwnersForGKV(gvk schema.GroupVersionKind) []dynamiccache.OwnerReference
}

type reconciler interface {
	Reconcile(ctx context.Context, pkg genericObjectTemplate) (ctrl.Result, error)
}

type preflightChecker interface {
	Check(
		ctx context.Context, owner, obj client.Object,
	) (violations []preflight.Violation, err error)
}

var _ environment.Sinker = (*GenericObjectTemplateController)(nil)

type GenericObjectTemplateController struct {
	newObjectTemplate  genericObjectTemplateFactory
	log                logr.Logger
	scheme             *runtime.Scheme
	client             client.Client
	uncachedClient     client.Client
	dynamicCache       dynamicCache
	templateReconciler *templateReconciler
	reconciler         []reconciler
}

func NewObjectTemplateController(
	client, uncachedClient client.Client,
	log logr.Logger,
	dynamicCache dynamicCache,
	scheme *runtime.Scheme,
	restMapper meta.RESTMapper,
) *GenericObjectTemplateController {
	return newGenericObjectTemplateController(
		client, uncachedClient, log, dynamicCache, scheme,
		restMapper, newGenericObjectTemplate)
}

func NewClusterObjectTemplateController(
	client, uncachedClient client.Client,
	log logr.Logger,
	dynamicCache dynamicCache,
	scheme *runtime.Scheme,
	restMapper meta.RESTMapper,
) *GenericObjectTemplateController {
	return newGenericObjectTemplateController(
		client, uncachedClient, log, dynamicCache, scheme,
		restMapper, newGenericClusterObjectTemplate)
}

func newGenericObjectTemplateController(
	client, uncachedClient client.Client,
	log logr.Logger,
	dynamicCache dynamicCache,
	scheme *runtime.Scheme,
	restMapper meta.RESTMapper,
	newObjectTemplate genericObjectTemplateFactory,
) *GenericObjectTemplateController {
	controller := &GenericObjectTemplateController{
		newObjectTemplate: newObjectTemplate,
		log:               log,
		scheme:            scheme,
		client:            client,
		uncachedClient:    uncachedClient,
		dynamicCache:      dynamicCache,
		templateReconciler: newTemplateReconciler(scheme, client, uncachedClient, dynamicCache,
			preflight.NewAPIExistence(
				restMapper,
				preflight.List{
					preflight.NewNoOwnerReferences(restMapper),
					preflight.NewEmptyNamespaceNoDefault(restMapper),
					preflight.NewNamespaceEscalation(restMapper),
				},
			),
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
		if err := controllers.FreeCacheAndRemoveFinalizer(
			ctx, c.client, objectTemplate.ClientObject(), c.dynamicCache); err != nil {
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
	ctx context.Context, objectTemplate genericObjectTemplate,
) error {
	objectTemplate.UpdatePhase()
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
			c.dynamicCache.Source(
				dynamiccache.NewEnqueueWatchingObjects(c.dynamicCache, objectTemplate, mgr.GetScheme()),
			),
		).
		Complete(c)
}
