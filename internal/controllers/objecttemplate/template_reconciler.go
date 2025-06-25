package objecttemplate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-logr/logr"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/jsonpath"
	"pkg.package-operator.run/boxcutter/managedcache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/environment"
	"package-operator.run/internal/preflight"
)

// Requeue every 30s to check if input sources exist now.
var defaultMissingResourceRetryInterval = 30 * time.Second

type templateReconciler struct {
	*environment.Sink
	scheme                        *runtime.Scheme
	client                        client.Writer
	uncachedClient                client.Reader
	accessManager                 managedcache.ObjectBoundAccessManager[client.Object]
	preflightChecker              preflightChecker
	optionalResourceRetryInterval time.Duration
	resourceRetryInterval         time.Duration
}

func newTemplateReconciler(
	scheme *runtime.Scheme,
	client client.Client,
	uncachedClient client.Reader,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	preflightChecker preflightChecker,
	optionalResourceRetryInterval time.Duration,
	resourceRetryInterval time.Duration,
) *templateReconciler {
	return &templateReconciler{
		Sink: environment.NewSink(client),

		scheme:                        scheme,
		client:                        client,
		uncachedClient:                uncachedClient,
		accessManager:                 accessManager,
		preflightChecker:              preflightChecker,
		optionalResourceRetryInterval: optionalResourceRetryInterval,
		resourceRetryInterval:         resourceRetryInterval,
	}
}

func (r *templateReconciler) Reconcile(
	ctx context.Context, objectTemplate adapters.ObjectTemplateAccessor,
) (res ctrl.Result, err error) {
	defer func() {
		err = setObjectTemplateConditionBasedOnError(objectTemplate, err)
	}()

	sourcesConfig := map[string]any{}
	retryLater, err := r.getValuesFromSources(ctx, objectTemplate, sourcesConfig)
	if err != nil {
		if isMissingResourceError(err) {
			res.RequeueAfter = r.resourceRetryInterval
		}
		return res, fmt.Errorf("retrieving values from sources: %w", err)
	}
	// For optional resources.
	if retryLater {
		res.RequeueAfter = r.optionalResourceRetryInterval
	}

	obj := &unstructured.Unstructured{
		Object: map[string]any{},
	}
	if err := r.templateObject(ctx, sourcesConfig, objectTemplate, obj); err != nil {
		return res, err
	}

	objects := []client.Object{
		objectTemplate.ClientObject(),
		obj,
	}
	cache, err := r.accessManager.GetWithUser(
		context.Background(),
		constants.StaticCacheOwner(),
		objectTemplate.ClientObject(),
		objects,
	)
	if err != nil {
		return res, err
	}

	existingObj := &unstructured.Unstructured{}
	existingObj.SetGroupVersionKind(obj.GroupVersionKind())
	if err := cache.Get(ctx, client.ObjectKeyFromObject(obj), existingObj); apimachineryerrors.IsNotFound(err) {
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
	obj.SetFinalizers(existingObj.GetFinalizers())
	obj.SetLabels(labels.Merge(existingObj.GetLabels(), obj.GetLabels()))
	obj.SetAnnotations(labels.Merge(existingObj.GetAnnotations(), obj.GetAnnotations()))

	obj.SetResourceVersion(existingObj.GetResourceVersion())
	if err := r.client.Update(ctx, obj); err != nil {
		return res, fmt.Errorf("updating templated object: %w", err)
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	controllerOf := corev1alpha1.ControlledObjectReference{
		Kind:      gvk.Kind,
		Group:     gvk.Group,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	objectTemplate.SetStatusControllerOf(controllerOf)

	if err := r.accessManager.FreeWithUser(
		ctx,
		constants.StaticCacheOwner(),
		objectTemplate.ClientObject(),
	); err != nil {
		return res, fmt.Errorf("freewithuser: %w", err)
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
	ctx context.Context, objectTemplate adapters.ObjectTemplateAccessor,
	sourcesConfig map[string]any,
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
		if err := copySourceItems(src.Items, sourceObj, sourcesConfig); err != nil {
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

	objects := []client.Object{
		objectTemplate,
		sourceObj,
	}
	cache, err := r.accessManager.GetWithUser(
		context.Background(),
		constants.StaticCacheOwner(),
		objectTemplate,
		objects,
	)
	if err != nil {
		return nil, false, err
	}

	objectKey := client.ObjectKeyFromObject(sourceObj)

	if err := cache.Get(ctx, objectKey, sourceObj); apimachineryerrors.IsNotFound(err) {
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
		updatedSourceObj, err := controllers.AddDynamicCacheLabel(ctx, r.client, sourceObj)
		if err != nil {
			return nil, false, fmt.Errorf("patching source object for cache: %w", err)
		}
		sourceObj = updatedSourceObj
	} else if err != nil {
		err := fmt.Errorf("getting source object %s in namespace %s: %w", objectKey.Name, objectKey.Namespace, err)
		return nil, false, err
	}
	return sourceObj, true, nil
}

func (r *templateReconciler) lookupUncached(
	ctx context.Context, src corev1alpha1.ObjectTemplateSource, key client.ObjectKey, obj client.Object,
) (found bool, err error) {
	if err := r.uncachedClient.Get(ctx, key, obj); apimachineryerrors.IsNotFound(err) {
		if src.Optional {
			// just skip this one if it's optional.
			return false, nil
		}
		return false, &SourceError{Source: obj, Err: err}
	} else if err != nil {
		err := fmt.Errorf("getting source object %s in namespace %s from uncachedClient: %w", key.Name, key.Namespace, err)
		return false, err
	}
	return true, nil
}

func copySourceItems(
	src []corev1alpha1.ObjectTemplateSourceItem,
	sourceObj *unstructured.Unstructured, sourcesConfig map[string]any,
) error {
	for _, item := range src {
		if err := copySourceItem(item, sourceObj, sourcesConfig); err != nil {
			return err
		}
	}
	return nil
}

func copySourceItem(
	item corev1alpha1.ObjectTemplateSourceItem,
	sourceObj *unstructured.Unstructured,
	sourcesConfig map[string]any,
) error {
	jpString, err := RelaxedJSONPathExpression(item.Key)
	if err != nil {
		return err
	}

	jp := jsonpath.New("key")
	jp.EnableJSONOutput(true)
	if err := jp.Parse(jpString); err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := jp.Execute(&buf, sourceObj.Object); err != nil {
		return err
	}
	var value any
	if err := json.Unmarshal(buf.Bytes(), &value); err != nil {
		return err
	}
	if vslice, ok := value.([]any); ok && len(vslice) == 1 {
		value = vslice[0]
	}

	if string(item.Destination[0]) != "." {
		return &JSONPathFormatError{Path: item.Destination}
	}
	trimmedDestination := strings.TrimPrefix(item.Destination, ".")
	if err := unstructured.SetNestedField(sourcesConfig, value, strings.Split(trimmedDestination, ".")...); err != nil {
		return fmt.Errorf("setting nested field at %s: %w", item.Destination, err)
	}

	return nil
}

func (r *templateReconciler) templateObject(
	ctx context.Context, sourcesConfig map[string]any,
	objectTemplate adapters.ObjectTemplateAccessor, object client.Object,
) error {
	env, err := r.getEnvironment(ctx, objectTemplate.ClientObject().GetNamespace())
	if err != nil {
		return fmt.Errorf("getting environment: %w", err)
	}
	templateContext := TemplateContext{
		Config:      sourcesConfig,
		Environment: env,
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
		return &SourceError{Source: object, Err: &preflight.Error{Violations: violations}}
	}

	if len(objectTemplate.ClientObject().GetNamespace()) > 0 {
		object.SetNamespace(objectTemplate.ClientObject().GetNamespace())
	}

	object.SetLabels(labels.Merge(object.GetLabels(), map[string]string{constants.DynamicCacheLabel: "True"}))

	return nil
}

func (r *templateReconciler) getEnvironment(ctx context.Context, namespace string) (map[string]any, error) {
	env, err := r.GetEnvironment(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("get environment: %w", err)
	}

	envData := map[string]any{}
	packageEnvironment, err := json.Marshal(env)
	if err != nil {
		return envData, fmt.Errorf("marshaling: %w", err)
	}

	err = json.Unmarshal(packageEnvironment, &envData)
	if err != nil {
		return envData, fmt.Errorf("unmarshaling: %w", err)
	}
	return envData, nil
}

func updateStatusConditionsFromOwnedObject(
	_ context.Context, objectTemplate adapters.ObjectTemplateAccessor, existingObj *unstructured.Unstructured,
) error {
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
		condMap, ok := cond.(map[string]any)
		if !ok {
			return apimachineryerrors.NewBadRequest("malformed condition")
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
		meta.SetStatusCondition(objectTemplate.GetStatusConditions(), newCond)
	}
	return nil
}

func setObjectTemplateConditionBasedOnError(objectTemplate adapters.ObjectTemplateAccessor, err error) error {
	var sourceError *SourceError
	if errors.As(err, &sourceError) {
		meta.SetStatusCondition(objectTemplate.GetStatusConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectTemplateInvalid,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: objectTemplate.GetGeneration(),
			Reason:             "SourceError",
			Message:            sourceError.Error(),
		})
		return nil // don't retry error
	}
	var templateError *TemplateError
	if errors.As(err, &templateError) {
		meta.SetStatusCondition(objectTemplate.GetStatusConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectTemplateInvalid,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: objectTemplate.GetGeneration(),
			Reason:             "TemplateError",
			Message:            templateError.Error(),
		})
		return nil // don't retry error
	}

	if err == nil {
		meta.RemoveStatusCondition(objectTemplate.GetStatusConditions(), corev1alpha1.ObjectTemplateInvalid)
	}
	return err
}

var jsonRegexp = regexp.MustCompile(`^\{\.?([^{}]+)\}$|^\.?([^{}]+)$`)

// RelaxedJSONPathExpression attempts to be flexible with JSONPath expressions, it accepts:
//   - metadata.name (no leading '.' or curly braces '{...}'
//   - {metadata.name} (no leading '.')
//   - .metadata.name (no curly braces '{...}')
//   - {.metadata.name} (complete expression)
//
// And transforms them all into a valid jsonpath expression:
//
//	{.metadata.name}
func RelaxedJSONPathExpression(pathExpression string) (string, error) {
	if len(pathExpression) == 0 {
		return pathExpression, nil
	}
	submatches := jsonRegexp.FindStringSubmatch(pathExpression)
	if submatches == nil {
		err := errors.New("path string, expected a 'name1.name2' or '.name1.name2' or '{name1.name2}' or '{.name1.name2}'")
		return "", err
	}
	if len(submatches) != 3 {
		return "", fmt.Errorf("unexpected submatch list: %v", submatches)
	}
	var fieldSpec string
	if len(submatches[1]) != 0 {
		fieldSpec = submatches[1]
	} else {
		fieldSpec = submatches[2]
	}
	return fmt.Sprintf("{.%s}", fieldSpec), nil
}

func isMissingResourceError(err error) bool {
	var sourceError *SourceError
	if errors.As(err, &sourceError) {
		return apimachineryerrors.IsNotFound(sourceError.Err)
	}
	return false
}
