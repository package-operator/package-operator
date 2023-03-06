package objecttemplate

import (
	"context"
	"fmt"
	"strings"

	"package-operator.run/package-operator/internal/utils"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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
	"package-operator.run/package-operator/internal/dynamiccache"
)

type dynamicCache interface {
	client.Reader
	Source() source.Source
	Free(ctx context.Context, obj client.Object) error
	Watch(ctx context.Context, owner client.Object, obj runtime.Object) error
	OwnersForGKV(gvk schema.GroupVersionKind) []dynamiccache.OwnerReference
}

type preflightChecker interface {
	CheckObj(
		ctx context.Context, owner,
		obj client.Object,
	) (violations []preflight.Violation, err error)
}

type GenericObjectTemplateController struct {
	newObjectTemplate genericObjectTemplateFactory
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
		log:               log,
		scheme:            scheme,
		dynamicCache:      dynamicCache,
		preflightChecker: preflight.List{
			preflight.NewAPIExistence(restMapper),
			preflight.NewEmptyNamespaceNoDefault(restMapper),
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
		log:               log,
		scheme:            scheme,
		client:            client,
		uncachedClient:    uncachedClient,
		dynamicCache:      dynamicCache,
		preflightChecker: preflight.List{
			preflight.NewAPIExistence(restMapper),
			preflight.NewEmptyNamespaceNoDefault(restMapper),
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

	sources := &unstructured.Unstructured{Object: map[string]interface{}{}}
	if err := c.getValuesFromSources(ctx, objectTemplate, sources); err != nil {
		return ctrl.Result{}, fmt.Errorf("retrieving values from sources: %w", err)
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}
	if err := c.templateObject(ctx, sources, objectTemplate, obj); err != nil {
		return ctrl.Result{}, err
	}
	existingObj := &unstructured.Unstructured{}
	existingObj.SetGroupVersionKind(obj.GroupVersionKind())

	if err := c.client.Get(ctx, client.ObjectKeyFromObject(obj), existingObj); err != nil {
		if errors.IsNotFound(err) {
			if err := c.handleCreation(ctx, objectTemplate.ClientObject(), obj); err != nil {
				return ctrl.Result{}, fmt.Errorf("handling creation: %w", err)
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting existing object: %w", err)
	}
	if err := c.updateStatusConditionsFromOwnedObject(ctx, objectTemplate, existingObj); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status conditions from owned object: %w", err)
	}

	obj.SetOwnerReferences(existingObj.GetOwnerReferences())
	obj.SetLabels(utils.MergeKeysFrom(existingObj.GetLabels(), obj.GetLabels()))
	obj.SetAnnotations(utils.MergeKeysFrom(existingObj.GetAnnotations(), obj.GetAnnotations()))
	obj.SetResourceVersion(existingObj.GetResourceVersion())
	return ctrl.Result{}, c.client.Update(ctx, obj)
}

func (c *GenericObjectTemplateController) handleCreation(ctx context.Context, owner, object client.Object) error {
	if err := controllerutil.SetControllerReference(owner, object, c.scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}

	if err := c.client.Create(ctx, object); err != nil {
		return fmt.Errorf("creating object: %w", err)
	}

	return nil
}

func (c *GenericObjectTemplateController) getValuesFromSources(ctx context.Context, objectTemplate genericObjectTemplate, sources *unstructured.Unstructured) error {
	for _, src := range objectTemplate.GetSources() {
		sourceObj := &unstructured.Unstructured{}
		sourceObj.SetName(src.Name)
		sourceObj.SetKind(src.Kind)
		sourceObj.SetAPIVersion(src.APIVersion)
		sourceObj.SetNamespace(src.Namespace)

		violations, err := c.preflightChecker.CheckObj(ctx, objectTemplate.ClientObject(), sourceObj)
		if err != nil {
			return err
		}
		if len(violations) > 0 {
			return &preflight.Error{Violations: violations}
		}

		if len(objectTemplate.ClientObject().GetNamespace()) > 0 {
			sourceObj.SetNamespace(objectTemplate.ClientObject().GetNamespace())
		}

		if err := c.dynamicCache.Watch(
			ctx, objectTemplate.ClientObject(), sourceObj); err != nil {
			return fmt.Errorf("watching new resource: %w", err)
		}

		objectKey := client.ObjectKeyFromObject(sourceObj)
		err = c.dynamicCache.Get(ctx, objectKey, sourceObj)
		if errors.IsNotFound(err) {
			// the referenced object might not be labeled correctly for the cache to pick up,
			// fallback to an uncached read to discover.
			if err := c.uncachedClient.Get(ctx, objectKey, sourceObj); err != nil {
				return fmt.Errorf("getting source object %s in namespace %s from uncachedClient: %w", objectKey.Name, objectKey.Namespace, err)
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

func (c *GenericObjectTemplateController) templateObject(ctx context.Context, sources *unstructured.Unstructured, objectTemplate genericObjectTemplate, object client.Object) error {
	templateContext := TemplateContext{
		Config: sources.Object,
	}
	transformer, err := NewTemplateTransformer(templateContext)
	if err != nil {
		return fmt.Errorf("creating transformer: %w", err)
	}
	renderedTemplate, err := transformer.transform(ctx, []byte(objectTemplate.GetTemplate()))
	if err != nil {
		return fmt.Errorf("rendering template: %w", err)
	}

	if err := yaml.Unmarshal(renderedTemplate, object); err != nil {
		return fmt.Errorf("unmarshalling yaml of rendered template: %w", err)
	}
	violations, err := c.preflightChecker.CheckObj(ctx, objectTemplate.ClientObject(), object)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return &preflight.Error{Violations: violations}
	}

	if len(objectTemplate.ClientObject().GetNamespace()) > 0 {
		object.SetNamespace(objectTemplate.ClientObject().GetNamespace())
	}

	return nil
}

func (c *GenericObjectTemplateController) updateStatusConditionsFromOwnedObject(ctx context.Context, objectTemplate genericObjectTemplate, existingObj *unstructured.Unstructured) error {
	statusObservedGeneration, _, err := unstructured.NestedInt64(existingObj.Object, "status", "observedGeneration")
	if err != nil {
		return fmt.Errorf("getting status observedGeneration: %w", err)
	}

	objectConds, found, err := unstructured.NestedSlice(existingObj.Object, "status", "conditions")
	if err != nil {
		return fmt.Errorf("getting conditions from object: %w", err)
	}

	if found {
		for _, cond := range objectConds {
			condMap, ok := cond.(map[string]interface{})
			if !ok {
				return errors.NewBadRequest("malformed condition")
			}

			condObservedGeneration, _, err := unstructured.NestedInt64(condMap, "observedGeneration")
			if err != nil {
				return fmt.Errorf("getting status observedGeneration: %w", err)
			}

			if existingObj.GetGeneration() != condObservedGeneration && existingObj.GetGeneration() != statusObservedGeneration {
				// condition is out of date, don't copy it over
				continue
			}

			message := fmt.Sprintf("Owned %s: %s", existingObj.GroupVersionKind().Kind, condMap["message"].(string))

			newCond := metav1.Condition{
				Type:               condMap["type"].(string),
				Status:             metav1.ConditionStatus(condMap["status"].(string)),
				ObservedGeneration: objectTemplate.ClientObject().GetGeneration(),
				Reason:             condMap["reason"].(string),
				Message:            message,
			}
			meta.SetStatusCondition(objectTemplate.GetConditions(), newCond)
		}

		if err := c.client.Status().Update(ctx, objectTemplate.ClientObject()); err != nil {
			return fmt.Errorf("updating objectTemplate status: %w", err)
		}
	}
	return nil
}

func (c *GenericObjectTemplateController) SetupWithManager(
	mgr ctrl.Manager,
) error {
	objectTemplate := c.newObjectTemplate(c.scheme).ClientObject()

	mapperFunc := func(obj client.Object) []reconcile.Request {
		owners := c.dynamicCache.OwnersForGKV(obj.GetObjectKind().GroupVersionKind())
		requests := make([]reconcile.Request, len(owners))
		for i, owner := range owners {
			requests[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      owner.Name,
					Namespace: owner.Namespace,
				},
			}
		}
		return requests
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(objectTemplate).
		Watches(c.dynamicCache.Source(), handler.EnqueueRequestsFromMapFunc(mapperFunc)).Complete(c)
}
