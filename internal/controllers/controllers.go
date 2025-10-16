package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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
