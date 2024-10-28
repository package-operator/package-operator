package objecthandling

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/constants"
)

// HasDynamicCacheLabel checks if the given client object has the dynamic cache label.
// It does not retrieve anything from the API.
func HasDynamicCacheLabel(obj client.Object) bool {
	labels := obj.GetLabels()
	value, ok := labels[constants.DynamicCacheLabel]
	return ok && value == "True"
}

// EnsureDynamicCacheLabel ensures that the given object is labeled
// for recognition by the dynamic cache.
func EnsureDynamicCacheLabel(ctx context.Context, w client.Writer, obj client.Object) error {
	// Return early if the cache label is already there.
	if HasDynamicCacheLabel(obj) {
		return nil
	}

	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[constants.DynamicCacheLabel] = "True"
	obj.SetLabels(labels)

	patch := map[string]any{
		"metadata": map[string]any{
			"resourceVersion": obj.GetResourceVersion(),
			"labels":          obj.GetLabels(),
		},
	}

	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshalling patch to ensure dynamic cache label: %w", err)
	}

	if err := w.Patch(ctx, obj, client.RawPatch(types.MergePatchType, patchJSON)); err != nil {
		return fmt.Errorf("patching dynamic cache label: %w", err)
	}
	return nil
}

// RemoveDynamicCacheLabel ensures that the given object does not bear
// the label for dynamic cache inclusion anymore.
// It returns the updated version of the object.
func RemoveDynamicCacheLabel(ctx context.Context, w client.Writer, obj client.Object) error {
	labels := obj.GetLabels()
	delete(labels, constants.DynamicCacheLabel)
	obj.SetLabels(labels)

	patch := map[string]any{
		"metadata": map[string]any{
			"resourceVersion": obj.GetResourceVersion(),
			"labels":          obj.GetLabels(),
		},
	}

	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshalling patch to remove dynamic cache label: %w", err)
	}

	if err := w.Patch(ctx, obj, client.RawPatch(types.MergePatchType, patchJSON)); err != nil {
		return fmt.Errorf("patching dynamic cache label: %w", err)
	}
	return nil
}
