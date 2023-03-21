package objecttemplate

import (
	"context"
	goerrors "errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/preflight"
	"package-operator.run/package-operator/internal/utils"
)

// Requeue every 30s to check if input sources exist now.
var defaultMissingResourceRetryInterval = 30 * time.Second

type templateReconciler struct {
	scheme           *runtime.Scheme
	client           client.Writer
	uncachedClient   client.Reader
	dynamicCache     dynamicCache
	preflightChecker preflightChecker
}

func newTemplateReconciler(
	scheme *runtime.Scheme,
	client client.Writer,
	uncachedClient client.Reader,
	dynamicCache dynamicCache,
	preflightChecker preflightChecker,
) *templateReconciler {
	return &templateReconciler{
		scheme:           scheme,
		client:           client,
		uncachedClient:   uncachedClient,
		dynamicCache:     dynamicCache,
		preflightChecker: preflightChecker,
	}
}

func (r *templateReconciler) Reconcile(
	ctx context.Context, objectTemplate genericObjectTemplate,
) (res ctrl.Result, err error) {
	defer func() {
		err = setObjectTemplateConditionBasedOnError(objectTemplate, err)
	}()

	sourcesConfig := map[string]interface{}{}
	retryLater, err := r.getValuesFromSources(ctx, objectTemplate, sourcesConfig)
	if err != nil {
		return res, fmt.Errorf("retrieving values from sources: %w", err)
	}
	if retryLater {
		res.RequeueAfter = defaultMissingResourceRetryInterval
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}
	if err := r.templateObject(ctx, sourcesConfig, objectTemplate, obj); err != nil {
		return res, err
	}

	if err := r.dynamicCache.Watch(
		ctx, objectTemplate.ClientObject(), obj); err != nil {
		return res, fmt.Errorf("watching new child: %w", err)
	}

	existingObj := &unstructured.Unstructured{}
	existingObj.SetGroupVersionKind(obj.GroupVersionKind())
	if err := r.dynamicCache.Get(ctx, client.ObjectKeyFromObject(obj), existingObj); errors.IsNotFound(err) {
		if err := r.handleCreation(ctx, objectTemplate.ClientObject(), obj); err != nil {
			return res, fmt.Errorf("handling creation: %w", err)
		}
		return res, nil
	} else if err != nil {
		return res, fmt.Errorf("getting existing object: %w", err)
	}
	if err := updateStatusConditionsFromOwnedObject(ctx, objectTemplate, existingObj); err != nil {
		return res, fmt.Errorf("updating status conditions from owned object: %w", err)
	}

	obj.SetOwnerReferences(existingObj.GetOwnerReferences())
	obj.SetLabels(utils.MergeKeysFrom(existingObj.GetLabels(), obj.GetLabels()))
	obj.SetAnnotations(utils.MergeKeysFrom(existingObj.GetAnnotations(), obj.GetAnnotations()))
	obj.SetResourceVersion(existingObj.GetResourceVersion())
	if err := r.client.Update(ctx, obj); err != nil {
		return res, fmt.Errorf("updating templated object: %w", err)
	}

	return res, nil
}

func (r *templateReconciler) handleCreation(ctx context.Context, owner, object client.Object) error {
	if err := controllerutil.SetControllerReference(owner, object, r.scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}

	if err := r.client.Create(ctx, object); err != nil {
		return fmt.Errorf("creating object: %w", err)
	}

	return nil
}

func (r *templateReconciler) getValuesFromSources(
	ctx context.Context, objectTemplate genericObjectTemplate,
	sourcesConfig map[string]interface{},
) (retryLater bool, err error) {
	log := logr.FromContextOrDiscard(ctx)
	for _, src := range objectTemplate.GetSources() {
		sourceObj, found, err := r.getSourceObject(ctx, objectTemplate.ClientObject(), src)
		if err != nil {
			return false, err
		}
		if !found {
			log.Info(fmt.Sprintf("optional source not found, retry in %s", defaultMissingResourceRetryInterval),
				"source", fmt.Sprintf("%s %s/%s", src.Kind, src.Namespace, src.Name))
			retryLater = true
			continue
		}
		if err := copySourceItems(ctx, src.Items, sourceObj, sourcesConfig); err != nil {
			return false, &SourceError{Source: sourceObj, Err: err}
		}
	}
	return retryLater, nil
}

func (r *templateReconciler) getSourceObject(
	ctx context.Context, objectTemplate client.Object,
	src corev1alpha1.ObjectTemplateSource,
) (sourceObj *unstructured.Unstructured, found bool, err error) {
	sourceObj = &unstructured.Unstructured{}
	sourceObj.SetName(src.Name)
	sourceObj.SetKind(src.Kind)
	sourceObj.SetAPIVersion(src.APIVersion)
	sourceObj.SetNamespace(src.Namespace)

	// Ensure we are staying within the same namespace.
	violations, err := r.preflightChecker.Check(ctx, objectTemplate, sourceObj)
	if err != nil {
		return nil, false, err
	}
	if len(violations) > 0 {
		return nil, false, &SourceError{Source: sourceObj, Err: &preflight.Error{Violations: violations}}
	}

	if len(sourceObj.GetNamespace()) == 0 {
		sourceObj.SetNamespace(objectTemplate.GetNamespace())
	}

	if err := r.dynamicCache.Watch(
		ctx, objectTemplate, sourceObj); err != nil {
		return nil, false, fmt.Errorf("watching new source: %w", err)
	}

	objectKey := client.ObjectKeyFromObject(sourceObj)

	if err := r.dynamicCache.Get(ctx, objectKey, sourceObj); errors.IsNotFound(err) {
		// the referenced object might not be labeled correctly for the cache to pick up,
		// fallback to an uncached read to discover.
		found, err := r.lookupUncached(ctx, src, objectKey, sourceObj)
		if err != nil {
			return nil, false, err
		}
		if !found {
			return nil, false, nil
		}

		// Update object to ensure it is part of our cache and we get events to reconcile.
		updatedSourceObj, err := addDynamicCacheLabel(ctx, sourceObj, r.client)
		if err != nil {
			return nil, false, fmt.Errorf("patching source object for cache: %w", err)
		}
		sourceObj = updatedSourceObj
	} else if err != nil {
		return nil, false, fmt.Errorf("getting source object %s in namespace %s: %w", objectKey.Name, objectKey.Namespace, err)
	}
	return sourceObj, true, nil
}

func (r *templateReconciler) lookupUncached(ctx context.Context, src corev1alpha1.ObjectTemplateSource, key client.ObjectKey, obj client.Object) (found bool, err error) {
	if err := r.uncachedClient.Get(ctx, key, obj); errors.IsNotFound(err) {
		if src.Optional {
			// just skip this one if it's optional.
			return false, nil
		}
		return false, &SourceError{Source: obj, Err: err}
	} else if err != nil {
		return false, fmt.Errorf("getting source object %s in namespace %s from uncachedClient: %w", key.Name, key.Namespace, err)
	}
	return true, nil
}

func addDynamicCacheLabel(
	ctx context.Context,
	sourceObj *unstructured.Unstructured,
	c client.Writer,
) (updatedObj *unstructured.Unstructured, err error) {
	// Update object to ensure it is part of our cache and we get events to reconcile.
	updatedSourceObj := sourceObj.DeepCopy()

	labels := updatedSourceObj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	labels[controllers.DynamicCacheLabel] = "True"
	updatedSourceObj.SetLabels(labels)

	if err := c.Patch(ctx, updatedSourceObj, client.MergeFrom(sourceObj)); err != nil {
		return nil, fmt.Errorf("patching source object for cache: %w", err)
	}
	return updatedSourceObj, nil
}

func copySourceItems(
	ctx context.Context, src []corev1alpha1.ObjectTemplateSourceItem,
	sourceObj *unstructured.Unstructured, sourcesConfig map[string]interface{},
) error {
	for _, item := range src {
		if string(item.Key[0]) != "." {
			return &JSONPathFormatError{Path: item.Key}
		}
		trimmedKey := strings.TrimPrefix(item.Key, ".")
		value, found, err := unstructured.NestedFieldCopy(sourceObj.Object, strings.Split(trimmedKey, ".")...)
		if err != nil {
			return fmt.Errorf("getting value at %s: %w", item.Key, err)
		}
		if !found {
			return &SourceKeyNotFoundError{Key: item.Key}
		}

		if string(item.Destination[0]) != "." {
			return &JSONPathFormatError{Path: item.Destination}
		}
		trimmedDestination := strings.TrimPrefix(item.Destination, ".")
		if err := unstructured.SetNestedField(sourcesConfig, value, strings.Split(trimmedDestination, ".")...); err != nil {
			return fmt.Errorf("setting nested field at %s: %w", item.Destination, err)
		}
	}
	return nil
}

func (r *templateReconciler) templateObject(
	ctx context.Context, sourcesConfig map[string]interface{},
	objectTemplate genericObjectTemplate, object client.Object,
) error {
	templateContext := TemplateContext{
		Config: sourcesConfig,
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
	violations, err := r.preflightChecker.Check(ctx, objectTemplate.ClientObject(), object)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return &preflight.Error{Violations: violations}
	}

	if len(objectTemplate.ClientObject().GetNamespace()) > 0 {
		object.SetNamespace(objectTemplate.ClientObject().GetNamespace())
	}
	object.SetLabels(utils.MergeKeysFrom(object.GetLabels(), map[string]string{
		controllers.DynamicCacheLabel: "True",
	}))
	return nil
}

func updateStatusConditionsFromOwnedObject(ctx context.Context, objectTemplate genericObjectTemplate, existingObj *unstructured.Unstructured) error {
	statusObservedGeneration, ok, err := unstructured.NestedInt64(existingObj.Object, "status", "observedGeneration")
	if err != nil {
		return fmt.Errorf("getting status observedGeneration: %w", err)
	}
	if ok &&
		statusObservedGeneration != objectTemplate.
			ClientObject().GetGeneration() {
		// all .status is outdated
		return nil
	}

	objectConds, found, err := unstructured.NestedSlice(existingObj.Object, "status", "conditions")
	if err != nil {
		return fmt.Errorf("getting conditions from object: %w", err)
	}

	if !found {
		return nil
	}
	for _, cond := range objectConds {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			return errors.NewBadRequest("malformed condition")
		}

		condObservedGeneration, _, err := unstructured.NestedInt64(condMap, "observedGeneration")
		if err != nil {
			return fmt.Errorf("getting status observedGeneration: %w", err)
		}

		if existingObj.GetGeneration() != condObservedGeneration {
			// condition is out of date, don't copy it over
			continue
		}

		newCond := metav1.Condition{
			Type:               condMap["type"].(string),
			Status:             metav1.ConditionStatus(condMap["status"].(string)),
			ObservedGeneration: objectTemplate.ClientObject().GetGeneration(),
			Reason:             condMap["reason"].(string),
			Message:            condMap["message"].(string),
		}
		meta.SetStatusCondition(objectTemplate.GetConditions(), newCond)
	}
	return nil
}

func setObjectTemplateConditionBasedOnError(objectTemplate genericObjectTemplate, err error) error {
	var sourceError *SourceError
	if goerrors.As(err, &sourceError) {
		meta.SetStatusCondition(objectTemplate.GetConditions(), metav1.Condition{
			Type:    corev1alpha1.ObjectTemplateInvalid,
			Status:  metav1.ConditionTrue,
			Reason:  "SourceError",
			Message: sourceError.Error(),
		})
		return nil // don't retry error
	}
	var templateError *TemplateError
	if goerrors.As(err, &templateError) {
		meta.SetStatusCondition(objectTemplate.GetConditions(), metav1.Condition{
			Type:    corev1alpha1.ObjectTemplateInvalid,
			Status:  metav1.ConditionTrue,
			Reason:  "TemplateError",
			Message: templateError.Error(),
		})
		return nil // don't retry error
	}

	if err == nil {
		meta.RemoveStatusCondition(objectTemplate.GetConditions(), corev1alpha1.ObjectTemplateInvalid)
	}
	return err
}
