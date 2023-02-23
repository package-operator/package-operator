package objecttemplate

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/yaml"

	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/packages/packageloader"
)

type dynamicCache interface {
	client.Reader
	Source() source.Source
	Free(ctx context.Context, obj client.Object) error
	Watch(ctx context.Context, owner client.Object, obj runtime.Object) error
}

type GenericObjectTemplateController struct {
	newObjectTemplate genericObjectTemplateFactory

	log            logr.Logger
	scheme         *runtime.Scheme
	client         client.Client
	uncachedClient client.Client
	dynamicCache   dynamicCache
}

func NewObjectTemplateController(
	client, uncachedClient client.Client,
	log logr.Logger,
	dynamicCache dynamicCache,
	scheme *runtime.Scheme,
) *GenericObjectTemplateController {
	return &GenericObjectTemplateController{
		client:            client,
		uncachedClient:    uncachedClient,
		newObjectTemplate: newGenericObjectTemplate,
		log:               log,
		scheme:            scheme,
		dynamicCache:      dynamicCache,
	}
}

func NewClusterObjectTemplateController(
	client, uncachedClient client.Client,
	log logr.Logger,
	dynamicCache dynamicCache,
	scheme *runtime.Scheme,
) *GenericObjectTemplateController {
	return &GenericObjectTemplateController{
		newObjectTemplate: newGenericClusterObjectTemplate,
		log:               log,
		scheme:            scheme,
		client:            client,
		uncachedClient:    uncachedClient,
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
	sources.Object = map[string]interface{}{}
	if err := c.GetValuesFromSources(ctx, objectTemplate, sources); err != nil {
		return ctrl.Result{}, fmt.Errorf("retrieving values from sources: %w", err)
	}

	pkg := &unstructured.Unstructured{}
	if err := c.TemplatePackage(ctx, objectTemplate.GetTemplate(), sources, pkg); err != nil {
		return ctrl.Result{}, err
	}
	existingPkg := &unstructured.Unstructured{}
	existingPkg.SetGroupVersionKind(pkg.GroupVersionKind())

	if err := c.client.Get(ctx, client.ObjectKeyFromObject(pkg), existingPkg); err != nil {
		if errors.IsNotFound(err) {
			if err := c.client.Create(ctx, pkg); err != nil {
				return ctrl.Result{}, fmt.Errorf("creating Package: %w", err)
			}
		}
		return ctrl.Result{}, fmt.Errorf("getting existing package: %w", err)
	}

	return ctrl.Result{}, c.client.Patch(ctx, pkg, client.MergeFrom(existingPkg))
}

func (c *GenericObjectTemplateController) GetValuesFromSources(ctx context.Context, objectTemplate genericObjectTemplate, sources *unstructured.Unstructured) error {
	for _, src := range objectTemplate.GetSources() {
		sourceObj := &unstructured.Unstructured{}
		sourceObj.SetName(src.Name)
		sourceObj.SetKind(src.Kind)
		sourceObj.SetAPIVersion(src.APIVersion)

		switch {
		case objectTemplate.ClientObject().GetNamespace() != "" && src.Namespace != "" && objectTemplate.ClientObject().GetNamespace() != src.Namespace:
			return errors.NewBadRequest(fmt.Sprintf("source %s references namespace %s, which is different from objectTemplate's namespace %s", src.Name, src.Namespace, objectTemplate.ClientObject().GetNamespace()))
		case objectTemplate.ClientObject().GetNamespace() != "":
			sourceObj.SetNamespace(objectTemplate.ClientObject().GetNamespace())
		case src.Namespace != "":
			sourceObj.SetNamespace(src.Namespace)
		default:
			return errors.NewBadRequest(fmt.Sprintf("neither the packageFile nor source object %s provides a namespace", sourceObj.GetName()))
		}

		if err := c.dynamicCache.Watch(
			ctx, objectTemplate.ClientObject(), sourceObj); err != nil {
			return fmt.Errorf("watching new resource: %w", err)
		}

		objectKey := client.ObjectKeyFromObject(sourceObj)
		err := c.dynamicCache.Get(ctx, objectKey, sourceObj)
		if errors.IsNotFound(err) {
			// the referenced secret might not be labeled correctly for the cache to pick up,
			// fallback to an uncached read to discover.
			if err := c.uncachedClient.Get(ctx, objectKey, sourceObj); err != nil {
				return fmt.Errorf("source object %s in namespace %s: %w", objectKey.Name, objectKey.Namespace, err)
			}

			// Update object to ensure it is part of our cache and we get events to reconcile.
			updatedSourceObj := sourceObj.DeepCopy()

			labels := updatedSourceObj.GetLabels()
			if labels == nil {
				labels = map[string]string{}
			}

			labels[controllers.DynamicCacheLabel] = "True"
			updatedSourceObj.SetLabels(labels)

			if err := c.client.Patch(ctx, updatedSourceObj, client.MergeFrom(sourceObj)); err != nil {
				return fmt.Errorf("patching source object for cache and ownership: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("getting source object %s in namespace %s: %w", objectKey.Name, objectKey.Namespace, err)
		}

		for _, item := range src.Items {
			value, found, err := unstructured.NestedFieldCopy(sourceObj.Object, strings.Split(item.Key, ".")...)
			if err != nil {
				return fmt.Errorf("getting value at %s from %s: %w", item.Key, sourceObj.GetName(), err)
			}
			if !found {
				return errors.NewBadRequest(fmt.Sprintf("source object %s does not have nested value at %s", sourceObj.GetName(), item.Key))
			}

			_, found, err = unstructured.NestedFieldNoCopy(sources.Object, strings.Split(item.Destination, ".")...)
			if err != nil {
				return fmt.Errorf("checking for duplicate destination at %s: %w", item.Destination, err)
			}
			if found {
				return fmt.Errorf("duplicate destination at %s: %w", item.Destination, err)
			}
			if err := unstructured.SetNestedField(sources.Object, value, strings.Split(item.Destination, ".")...); err != nil {
				return fmt.Errorf("setting nested field at %s: %w", item.Destination, err)
			}
		}
	}
	return nil
}

func (c *GenericObjectTemplateController) TemplatePackage(ctx context.Context, pkgTemplate string, sources *unstructured.Unstructured, pkg *unstructured.Unstructured) error {
	templateContext := packageloader.TemplateContext{
		Config: sources.Object,
	}
	transformer, err := packageloader.NewTemplateTransformer(templateContext)
	if err != nil {
		return fmt.Errorf("creating new packageFile transformer: %w", err)
	}
	fileMap := map[string][]byte{"package.gotmpl": []byte(pkgTemplate)}
	if err := transformer.TransformPackageFiles(ctx, fileMap); err != nil {
		return fmt.Errorf("rendering packageFile: %w", err)
	}

	if err := yaml.Unmarshal(fileMap["package"], pkg); err != nil {
		return fmt.Errorf("unmarshalling yaml of rendered packageFile: %w", err)
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
