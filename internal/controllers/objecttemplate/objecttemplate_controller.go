package objecttemplate

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"package-operator.run/package-operator/internal/preflight"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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

type preflightChecker interface {
	CheckObj(
		ctx context.Context, owner,
		obj client.Object,
	) (violations []preflight.Violation, err error)
}

// TODO: Move this to a shared space.
type PreflightError struct {
	Violations []preflight.Violation
}

func (e *PreflightError) Error() string {
	var vs []string
	for _, v := range e.Violations {
		vs = append(vs, v.String())
	}
	return strings.Join(vs, ", ")
}

type GenericObjectTemplateController struct {
	newObjectTemplate genericObjectTemplateFactory
	newPackage        genericPackageFactory
	log               logr.Logger
	scheme            *runtime.Scheme
	client            client.Client
	uncachedClient    client.Client
	dynamicCache      dynamicCache
	preflightChecker  preflightChecker
}

func NewObjectTemplateController(
	client, uncachedClient client.Client,
	log logr.Logger,
	dynamicCache dynamicCache,
	scheme *runtime.Scheme,
	restMapper meta.RESTMapper,
) *GenericObjectTemplateController {
	return &GenericObjectTemplateController{
		client:            client,
		uncachedClient:    uncachedClient,
		newObjectTemplate: newGenericObjectTemplate,
		newPackage:        newGenericPackage,
		log:               log,
		scheme:            scheme,
		dynamicCache:      dynamicCache,
		preflightChecker: preflight.List{
			preflight.NewAPIExistence(restMapper),
			preflight.NewNamespaceEscalation(restMapper),
		},
	}
}

func NewClusterObjectTemplateController(
	client, uncachedClient client.Client,
	log logr.Logger,
	dynamicCache dynamicCache,
	scheme *runtime.Scheme,
	restMapper meta.RESTMapper,
) *GenericObjectTemplateController {
	return &GenericObjectTemplateController{
		newObjectTemplate: newGenericClusterObjectTemplate,
		newPackage:        newGenericClusterPackage,
		log:               log,
		scheme:            scheme,
		client:            client,
		uncachedClient:    uncachedClient,
		dynamicCache:      dynamicCache,
		preflightChecker: preflight.List{
			preflight.NewAPIExistence(restMapper),
			preflight.NewNamespaceEscalation(restMapper),
		},
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

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}
	if err := c.TemplateObject(ctx, objectTemplate, sources, obj); err != nil {
		return ctrl.Result{}, err
	}
	existingObj := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	if err := c.client.Get(ctx, client.ObjectKeyFromObject(obj), existingObj); err != nil {
		if errors.IsNotFound(err) {
			if err := c.handleCreation(ctx, objectTemplate.ClientObject(), obj); err != nil {
				return ctrl.Result{}, fmt.Errorf("handling creation: %w", err)
			}
		}
		return ctrl.Result{}, fmt.Errorf("getting existing package: %w", err)
	}
	objectConds, found, err := unstructured.NestedSlice(existingObj.Object, "status", "conditions")
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting conditions from object: %w", err)
	}
	if found {
		for _, cond := range objectConds {
			condObj := cond.(metav1.Condition) // TODO: handle not ok?
			meta.SetStatusCondition(objectTemplate.GetConditions(), condObj)
		}
	}

	return ctrl.Result{}, c.client.Patch(ctx, existingObj, client.MergeFrom(obj))
}

func (c *GenericObjectTemplateController) handleCreation(ctx context.Context, owner, packageObject client.Object) error {
	if err := controllerutil.SetControllerReference(owner, packageObject, c.scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}

	if err := c.client.Create(ctx, packageObject); err != nil {
		return fmt.Errorf("creating Package: %w", err)
	}
	return nil
}

func (c *GenericObjectTemplateController) GetValuesFromSources(ctx context.Context, objectTemplate genericObjectTemplate, sources *unstructured.Unstructured) error {
	for _, src := range objectTemplate.GetSources() {
		sourceObj := &unstructured.Unstructured{}
		sourceObj.SetName(src.Name)
		sourceObj.SetKind(src.Kind)
		sourceObj.SetAPIVersion(src.APIVersion)

		// TODO: Should we move this to the namespace escilation part of the preflight checker?
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
			// the referenced object might not be labeled correctly for the cache to pick up,
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

func (c *GenericObjectTemplateController) TemplateObject(ctx context.Context, objectTemplate genericObjectTemplate, sources *unstructured.Unstructured, object client.Object) error {
	templateContext := packageloader.TemplateContext{
		Config: sources.Object,
	}
	transformer, err := packageloader.NewTemplateTransformer(templateContext)
	if err != nil {
		return fmt.Errorf("creating new packageFile transformer: %w", err)
	}
	fileMap := map[string][]byte{"object.gotmpl": []byte(objectTemplate.GetTemplate())}
	if err := transformer.TransformPackageFiles(ctx, fileMap); err != nil {
		return fmt.Errorf("rendering packageFile: %w", err)
	}

	if err := yaml.Unmarshal(fileMap["object"], object); err != nil {
		return fmt.Errorf("unmarshalling yaml of rendered packageFile: %w", err)
	}
	violations, err := c.preflightChecker.CheckObj(ctx, objectTemplate.ClientObject(), object)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return &PreflightError{Violations: violations}
	}

	if len(objectTemplate.ClientObject().GetNamespace()) > 0 {
		object.SetNamespace(objectTemplate.ClientObject().GetNamespace())
	}

	return nil
}

func (c *GenericObjectTemplateController) SetupWithManager(
	mgr ctrl.Manager,
) error {
	objectTemplate := c.newObjectTemplate(c.scheme).ClientObject()
	pkg := c.newPackage(c.scheme).ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(objectTemplate).
		Owns(pkg).
		Watches(c.dynamicCache.Source(), &handler.EnqueueRequestForOwner{
			OwnerType:    objectTemplate,
			IsController: false,
		}).Complete(c)
}
