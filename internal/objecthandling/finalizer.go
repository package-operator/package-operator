package objecthandling

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"package-operator.run/internal/constants"
)

// Ensures the given finalizer is set and persisted on the given object.
func EnsureFinalizer(
	ctx context.Context, c client.Writer,
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
	ctx context.Context, c client.Writer,
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
	ctx context.Context, c client.Writer, obj client.Object,
) error {
	return EnsureFinalizer(ctx, c, obj, constants.CachedFinalizer)
}

type cacheFreer interface {
	Free(ctx context.Context, obj client.Object) error
}

// Frees caches and removes the associated finalizer.
func FreeCacheAndRemoveFinalizer(
	ctx context.Context, c client.Writer,
	obj client.Object, cache cacheFreer,
) error {
	if err := cache.Free(ctx, obj); err != nil {
		return fmt.Errorf("free cache: %w", err)
	}

	return RemoveFinalizer(ctx, c, obj, constants.CachedFinalizer)
}
