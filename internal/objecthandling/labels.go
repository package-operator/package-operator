package objecthandling

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/controllers"
)

// HasDynamicCacheLabel checks if the given client object has the dynamic cache label.
// It does not retrieve anything from the API.
func HasDynamicCacheLabel(obj client.Object) bool {
	labels := obj.GetLabels()
	value, ok := labels[controllers.DynamicCacheLabel]
	return ok && value == "True"
}

// AddDynamicCacheLabel ensures that the given object is labeled
// for recognition by the dynamic cache.
func AddDynamicCacheLabel(ctx context.Context, w client.Writer, obj client.Object) error {
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())

	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	labels[controllers.DynamicCacheLabel] = "True"
	updated.SetLabels(labels)

	if err := w.Patch(ctx, updated,
		client.Apply, client.ForceOwnership, client.FieldOwner(controllers.FieldOwner)); err != nil {
		return fmt.Errorf("patching dynamic cache label: %w", err)
	}

	return nil
}

// RemoveDynamicCacheLabel ensures that the given object does not bear
// the label for dynamic cache inclusion anymore.
// It returns the updated version of the object.
func RemoveDynamicCacheLabel(ctx context.Context, w client.Writer, obj client.Object) error {
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())

	labels := obj.GetLabels()

	delete(labels, controllers.DynamicCacheLabel)
	updated.SetLabels(labels)

	if err := w.Patch(ctx, updated,
		client.Apply, client.ForceOwnership, client.FieldOwner(controllers.FieldOwner)); err != nil {
		return fmt.Errorf("patching dynamic cache label: %w", err)
	}

	return nil
}
