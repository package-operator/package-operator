package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

const (
	// This label is set on all dynamic objects to limit caches.
	DynamicCacheLabel = "package-operator.run/cache"
	// Common finalizer to free allocated caches when objects are deleted.
	CachedFinalizer = "package-operator.run/cached"
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
	return nil
}

func EnsureCachedFinalizer(
	ctx context.Context, c client.Client, obj client.Object,
) error {
	return EnsureFinalizer(ctx, c, obj, CachedFinalizer)
}

type cacheFreer interface {
	Free(ctx context.Context, obj client.Object) error
}

// Frees caches and removes the associated finalizer.
func FreeCacheAndRemoveFinalizer(
	ctx context.Context, c client.Client,
	obj client.Object, cache cacheFreer,
) error {
	if err := cache.Free(ctx, obj); err != nil {
		return fmt.Errorf("free cache: %w", err)
	}

	return RemoveFinalizer(ctx, c, obj, CachedFinalizer)
}

type isControllerChecker interface {
	IsController(owner, obj metav1.Object) bool
}

// Returns a list of ActiveObjectReferences controlled by this instance.
func FilterOwnActiveObjects(
	ctx context.Context, scheme *runtime.Scheme, ownerStrategy isControllerChecker,
	owner client.Object, actualObjects []client.Object,
) (activeObjects []corev1alpha1.ActiveObjectReference, err error) {
	var ownActiveObjects []corev1alpha1.ActiveObjectReference
	for _, actualObj := range actualObjects {
		if !ownerStrategy.IsController(owner, actualObj) {
			continue
		}

		gvk, err := apiutil.GVKForObject(actualObj, scheme)
		if err != nil {
			return nil, err
		}
		ownActiveObjects = append(ownActiveObjects, corev1alpha1.ActiveObjectReference{
			Kind:      gvk.Kind,
			Group:     gvk.Group,
			Name:      actualObj.GetName(),
			Namespace: actualObj.GetNamespace(),
		})
	}
	return ownActiveObjects, nil
}
