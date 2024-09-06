package objecthandling

import (
	"context"
	"fmt"

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

	if err := w.Patch(ctx, obj,
		client.Apply, client.ForceOwnership, client.FieldOwner(constants.FieldOwner)); err != nil {
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

	if err := w.Patch(ctx, obj,
		client.Apply, client.ForceOwnership, client.FieldOwner(constants.FieldOwner)); err != nil {
		return fmt.Errorf("patching dynamic cache label: %w", err)
	}

	return nil
}
