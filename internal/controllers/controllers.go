package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"pkg.package-operator.run/boxcutter/machinery"
	"pkg.package-operator.run/boxcutter/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"package-operator.run/internal/preflight"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/constants"
)

// Ensures the given finalizer is set and persisted on the given object.
func EnsureFinalizer(
	ctx context.Context, c client.Client,
	obj client.Object, finalizer string,
) error {
	if controllerutil.ContainsFinalizer(obj, finalizer) {
		return nil
	}

	controllerutil.AddFinalizer(obj, finalizer)
	patch := map[string]any{
		"metadata": map[string]any{
			"resourceVersion": obj.GetResourceVersion(),
			"finalizers":      obj.GetFinalizers(),
		},
	}
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshalling patch to remove finalizer: %w", err)
	}

	if err := c.Patch(ctx, obj, client.RawPatch(types.MergePatchType, patchJSON)); err != nil {
		return fmt.Errorf("adding finalizer: %w", err)
	}
	return nil
}

// Removes the given finalizer and persists the change.
func RemoveFinalizer(
	ctx context.Context, c client.Client,
	obj client.Object, finalizer string,
) error {
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		return nil
	}

	controllerutil.RemoveFinalizer(obj, finalizer)

	patch := map[string]any{
		"metadata": map[string]any{
			"resourceVersion": obj.GetResourceVersion(),
			"finalizers":      obj.GetFinalizers(),
		},
	}
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshalling patch to remove finalizer: %w", err)
	}
	if err := c.Patch(ctx, obj, client.RawPatch(types.MergePatchType, patchJSON)); err != nil {
		return fmt.Errorf("removing finalizer: %w", err)
	}
	return nil
}

func EnsureCachedFinalizer(
	ctx context.Context, c client.Client, obj client.Object,
) error {
	return EnsureFinalizer(ctx, c, obj, constants.CachedFinalizer)
}

// Removes the associated cache finalizer.
func RemoveCacheFinalizer(
	ctx context.Context, c client.Client,
	obj client.Object,
) error {
	return RemoveFinalizer(ctx, c, obj, constants.CachedFinalizer)
}

type isControllerChecker interface {
	IsController(owner, obj metav1.Object) bool
}

// Returns a list of ControlledObjectReferences controlled by this instance.
func GetStatusControllerOf(
	_ context.Context, scheme *runtime.Scheme, ownerStrategy isControllerChecker,
	owner client.Object, actualObjects []client.Object,
) ([]corev1alpha1.ControlledObjectReference, error) {
	controllerOf := make([]corev1alpha1.ControlledObjectReference, 0, len(actualObjects))
	for _, actualObj := range actualObjects {
		if !ownerStrategy.IsController(owner, actualObj) {
			continue
		}

		gvk, err := apiutil.GVKForObject(actualObj, scheme)
		if err != nil {
			return nil, err
		}
		controllerOf = append(controllerOf, corev1alpha1.ControlledObjectReference{
			Kind:      gvk.Kind,
			Group:     gvk.Group,
			Name:      actualObj.GetName(),
			Namespace: actualObj.GetNamespace(),
		})
	}
	return controllerOf, nil
}

func IsMappedCondition(cond metav1.Condition) bool {
	return strings.Contains(cond.Type, "/")
}

func MapConditions(
	_ context.Context,
	srcGeneration int64, srcConditions []metav1.Condition,
	destGeneration int64, destConditions *[]metav1.Condition,
) {
	for _, condition := range srcConditions {
		if condition.ObservedGeneration != srcGeneration {
			// mapped condition is outdated
			continue
		}

		if !IsMappedCondition(condition) {
			// mapped conditions are prefixed
			continue
		}

		meta.SetStatusCondition(destConditions, metav1.Condition{
			Type:               condition.Type,
			Status:             condition.Status,
			Reason:             condition.Reason,
			Message:            condition.Message,
			ObservedGeneration: destGeneration,
		})
	}
}

func DeleteMappedConditions(_ context.Context, conditions *[]metav1.Condition) {
	for _, cond := range *conditions {
		if IsMappedCondition(cond) {
			meta.RemoveStatusCondition(conditions, cond.Type)
		}
	}
}

// AddDynamicCacheLabel ensures that the given object is labeled
// for recognition by the dynamic cache.
func AddDynamicCacheLabel(
	ctx context.Context, w client.Writer, obj *unstructured.Unstructured,
) (*unstructured.Unstructured, error) {
	updated := obj.DeepCopy()

	labels := updated.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	labels[constants.DynamicCacheLabel] = "True"
	updated.SetLabels(labels)

	if err := w.Patch(ctx, updated, client.Merge); err != nil {
		return nil, fmt.Errorf("patching dynamic cache label: %w", err)
	}

	return updated, nil
}

func RemoveDynamicCacheLabel(
	ctx context.Context, w client.Writer, obj *unstructured.Unstructured,
) (*unstructured.Unstructured, error) {
	updated := obj.DeepCopy()

	labels := updated.GetLabels()

	delete(labels, constants.DynamicCacheLabel)
	updated.SetLabels(labels)

	if err := w.Patch(ctx, updated, client.MergeFrom(obj)); err != nil {
		return nil, fmt.Errorf("patching object labels: %w", err)
	}

	return updated, nil
}

type ObjectSetOrPhase interface {
	ClientObject() client.Object
	GetStatusConditions() *[]metav1.Condition
}

func UpdateObjectSetOrPhaseStatusFromError(
	ctx context.Context, objectSetOrPhase ObjectSetOrPhase,
	reconcileErr error, updateStatus func(ctx context.Context) error,
) (res ctrl.Result, err error) {
	var preflightError *preflight.Error
	if errors.As(reconcileErr, &preflightError) {
		meta.SetStatusCondition(objectSetOrPhase.GetStatusConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetAvailable,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: objectSetOrPhase.ClientObject().GetGeneration(),
			Reason:             "PreflightError",
			Message:            preflightError.Error(),
		})
		// Retry every once and a while to automatically unblock, if the preflight check issue has been cleared.
		res.RequeueAfter = DefaultGlobalMissConfigurationRetry
		return res, updateStatus(ctx)
	}

	var phaseValidationError *validation.PhaseValidationError
	if errors.As(reconcileErr, &phaseValidationError) {
		meta.SetStatusCondition(objectSetOrPhase.GetStatusConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetAvailable,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: objectSetOrPhase.ClientObject().GetGeneration(),
			Reason:             "PreflightError",
			Message:            phaseValidationError.Error(),
		})
		// Retry every once and a while to automatically unblock, if the preflight check issue has been cleared.
		res.RequeueAfter = DefaultGlobalMissConfigurationRetry
		return res, updateStatus(ctx)
	}

	if IsAdoptionRefusedError(reconcileErr) {
		meta.SetStatusCondition(objectSetOrPhase.GetStatusConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetAvailable,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: objectSetOrPhase.ClientObject().GetGeneration(),
			Reason:             "CollisionDetected",
			Message:            reconcileErr.Error(),
		})
		// Retry every once and a while to automatically unblock, if the conflicting resource has been deleted.
		res.RequeueAfter = DefaultGlobalMissConfigurationRetry
		return res, updateStatus(ctx)
	}

	// if we don't handle the error in any special way above,
	// just return it unchanged.
	return res, reconcileErr
}

func MapConditionsToObjectSetOrPhase(
	actualObjects []machinery.Object,
	ownerObjects []corev1alpha1.ObjectSetObject,
	objectSetOrPhase ObjectSetOrPhase,
) error {
	for _, obj := range actualObjects {
		unstructuredObj := obj.(*unstructured.Unstructured)

		rawConditions, exist, err := unstructured.NestedFieldNoCopy(
			unstructuredObj.Object, "status", "conditions")
		if err != nil {
			return err
		}
		if !exist {
			continue
		}

		j, err := json.Marshal(rawConditions)
		if err != nil {
			return err
		}
		var objectConditions []metav1.Condition
		if err := json.Unmarshal(j, &objectConditions); err != nil {
			return err
		}

		var conditionMappings []corev1alpha1.ConditionMapping
		for _, objectsetobject := range ownerObjects {
			if objectsetobject.Object.GetName() == obj.GetName() {
				conditionMappings = objectsetobject.ConditionMappings
			}
		}

		// Maps from object condition type to PKO condition type.
		conditionTypeMap := map[string]string{}
		for _, m := range conditionMappings {
			conditionTypeMap[m.SourceType] = m.DestinationType
		}
		for _, condition := range objectConditions {
			if condition.ObservedGeneration != 0 &&
				condition.ObservedGeneration != obj.GetGeneration() {
				// condition outdated
				continue
			}

			destType, ok := conditionTypeMap[condition.Type]
			if !ok {
				// condition not mapped
				continue
			}

			meta.SetStatusCondition(objectSetOrPhase.GetStatusConditions(), metav1.Condition{
				Type:               destType,
				Status:             condition.Status,
				Reason:             condition.Reason,
				Message:            condition.Message,
				ObservedGeneration: objectSetOrPhase.ClientObject().GetGeneration(),
			})
		}
	}

	return nil
}
