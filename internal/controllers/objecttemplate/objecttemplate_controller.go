package objecttemplate

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"package-operator.run/package-operator/internal/controllers"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
	"text/template"
)

type dynamicCache interface {
	client.Reader
	Source() source.Source
	Free(ctx context.Context, obj client.Object) error
	Watch(ctx context.Context, owner client.Object, obj runtime.Object) error
}

type GenericObjectTemplateController struct {
	newObjectTemplate genericObjectTemplateFactory

	log          logr.Logger
	scheme       *runtime.Scheme
	client       client.Client
	dynamicCache dynamicCache
}

func NewObjectTemplateController(
	client client.Client,
	log logr.Logger,
	dynamicCache dynamicCache,
	scheme *runtime.Scheme,
) *GenericObjectTemplateController {
	return &GenericObjectTemplateController{
		client:            client,
		newObjectTemplate: newGenericObjectTemplate,
		log:               log,
		scheme:            scheme,
		dynamicCache:      dynamicCache,
	}
}

func NewClusterObjectTemplateController(
	client client.Client,
	log logr.Logger,
	dynamicCache dynamicCache,
	scheme *runtime.Scheme,
) *GenericObjectTemplateController {
	return &GenericObjectTemplateController{
		newObjectTemplate: newGenericClusterObjectTemplate,
		log:               log,
		scheme:            scheme,
		client:            client,
		dynamicCache:      dynamicCache,
	}
}

func (c *GenericObjectTemplateController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("ObjectTemplate", req.String())
	defer log.Info("reconciled")
	ctx = logr.NewContext(ctx, log)

	defer log.Info("reconciled")

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
	}

	if err := controllers.EnsureCachedFinalizer(ctx, c.client, objectTemplate.ClientObject()); err != nil {
		return ctrl.Result{}, err
	}

	sources := &unstructured.Unstructured{}
	if err := c.GetValuesFromSources(ctx, objectTemplate, sources); err != nil {
		return ctrl.Result{}, err // TODO: expand error
	}

	t, err := template.New("objectTemplate").Parse(objectTemplate.GetTemplate())
	if err != nil {
		return ctrl.Result{}, err // TODO: expand error
	}
	buf := new(bytes.Buffer)
	if err = t.Execute(buf, sources); err != nil {
		return ctrl.Result{}, err // TODO: expand error
	}
	pkg := &unstructured.Unstructured{}
	if err := pkg.UnmarshalJSON(buf.Bytes()); err != nil {
		return ctrl.Result{}, err // TODO: expand error
	}

	existingPkg := &unstructured.Unstructured{}
	if err := c.client.Get(ctx, client.ObjectKeyFromObject(pkg), existingPkg); err != nil {
		if errors.IsNotFound(err) {
			if err := c.client.Create(ctx, pkg); err != nil {
				return ctrl.Result{}, fmt.Errorf("creating Package: %w", err)
			}
			return ctrl.Result{}, err // TODO: expand error
		}
	}

	if !reflect.DeepEqual(pkg, existingPkg) {
		return ctrl.Result{}, c.client.Update(ctx, pkg)
	}
	return ctrl.Result{}, nil
}

func (c *GenericObjectTemplateController) GetValuesFromSources(ctx context.Context, objectTemplate genericObjectTemplate, sources *unstructured.Unstructured) error {
	for _, source := range objectTemplate.GetSources() {
		sourceObj := &unstructured.Unstructured{}
		sourceObj.SetName(source.Name)
		sourceObj.SetKind(source.Kind)
		if objectTemplate.ClientObject().GetNamespace() != "" {
			sourceObj.SetNamespace(objectTemplate.ClientObject().GetNamespace())
		} else if source.Namespace != "" {
			sourceObj.SetNamespace(source.Namespace)
		} else {
			return nil // implement error
		}

		// Ensure to watch this type of object.
		if err := c.dynamicCache.Watch(
			ctx, objectTemplate.ClientObject(), sourceObj); err != nil {
			return fmt.Errorf("watching new resource: %w", err)
		}

		if err := c.dynamicCache.Get(ctx, client.ObjectKeyFromObject(sourceObj), sourceObj); err != nil {
			return err // TODO: expand error
		}

		for _, item := range source.Items {
			value, found, err := unstructured.NestedFieldCopy(sourceObj.Object, strings.Split(item.Key, ".")...)
			if err != nil {
				return err // TODO: expand error
			}
			if !found {
				return nil // TODO: return error that the key was not found
			}
			if err := unstructured.SetNestedField(sources.Object, value, strings.Split(item.Key, ".")...); err != nil {
				return err // TODO: expand error
			}
		}
	}
	return nil
}

func (c *GenericObjectTemplateController) SetupWithManager(
	mgr ctrl.Manager,
) error {
	objectTemplate := c.newObjectTemplate(c.scheme).ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(objectTemplate).
		Watches(c.dynamicCache.Source(), &handler.EnqueueRequestForOwner{
			OwnerType:    objectTemplate,
			IsController: false,
		}).Complete(c)
}
