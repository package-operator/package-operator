package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// This label is set on all dynamic objects to limit caches.
	DynamicCacheLabel = "package-operator.run/cache"
)

// Ensures the given finalizer is set and persisted on the given object.
func EnsureFinalizer(
	ctx context.Context, c client.Client,
	obj client.Object, finalizer string,
) error {
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		controllerutil.AddFinalizer(obj, finalizer)
		if err := c.Update(ctx, obj); err != nil {
			return fmt.Errorf("adding finalizer: %w", err)
		}
	}
	return nil
}

// Removes the given finalizer and persist the change.
func RemoveFinalizer(
	ctx context.Context, c client.Client,
	obj client.Object, finalizer string,
) error {
	if controllerutil.ContainsFinalizer(obj, finalizer) {
		controllerutil.RemoveFinalizer(obj, finalizer)

		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
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
	}
	return nil
}

type dynamicWatchFreer interface {
	Free(ctx context.Context, obj client.Object) error
}

// Frees caches and removes the associated finalizer.
func FreeCacheAndFinalizer(
	ctx context.Context, obj client.Object,
	c client.Client, dw dynamicWatchFreer,
	cacheFinalizer string,
) error {
	if err := dw.Free(ctx, obj); err != nil {
		return fmt.Errorf("free cache: %w", err)
	}

	return RemoveFinalizer(ctx, c, obj, cacheFinalizer)
}
