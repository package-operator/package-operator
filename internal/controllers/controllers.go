package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// This label is set on all dynamic objects, to limit caches.
	DynamicCacheLabel = "package-operator.run/cache"
	// Revision annotations holds a revision generation number to order ObjectSets.
	RevisionAnnotation = "package-operator.run/revision"
)

func EnsureCommonFinalizer(
	ctx context.Context, obj client.Object,
	c client.Client, cacheFinalizer string,
) error {
	if !controllerutil.ContainsFinalizer(obj, cacheFinalizer) {
		controllerutil.AddFinalizer(obj, cacheFinalizer)
		if err := c.Update(ctx, obj); err != nil {
			return fmt.Errorf("adding finalizer: %w", err)
		}
	}
	return nil
}

type dynamicWatchFreer interface {
	Free(ctx context.Context, obj client.Object) error
}

func HandleCommonDeletion(
	ctx context.Context, obj client.Object,
	c client.Client, dw dynamicWatchFreer,
	cacheFinalizer string,
) error {
	if err := dw.Free(ctx, obj); err != nil {
		return fmt.Errorf("free cache: %w", err)
	}

	if controllerutil.ContainsFinalizer(obj, cacheFinalizer) {
		controllerutil.RemoveFinalizer(obj, cacheFinalizer)

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

func GetObjectRevision(obj client.Object) (int64, error) {
	a := obj.GetAnnotations()
	if a == nil {
		return 0, nil
	}

	return strconv.ParseInt(a[RevisionAnnotation], 10, 64)
}

func SetObjectRevision(obj client.Object, revision int64) {
	a := obj.GetAnnotations()
	if a == nil {
		a = map[string]string{}
	}
	a[RevisionAnnotation] = fmt.Sprintf("%d", revision)
	obj.SetAnnotations(a)
}
