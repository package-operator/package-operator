package secretsync

import (
	"context"
	"fmt"
	"reflect"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/objecthandling"
)

const managedByLabel = "package-operator.run/managed-by-secretsync"

var _ reconciler = (*deletionReconciler)(nil)

type deletionReconciler struct {
	client       client.Client
	dynamicCache dynamicCache
}

func makeCoreV1SecretTypeMeta() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Secret",
	}
}

func (r *deletionReconciler) Reconcile(ctx context.Context, secretSync *v1alpha1.SecretSync) (reconcileResult, error) {
	// Return early if object is live.
	if secretSync.DeletionTimestamp.IsZero() {
		return reconcileResult{}, nil
	}

	// Take care of potential cleanup when object is deleting.
	switch {
	// Nothing to do if sync strategy is "poll".
	case secretSync.Spec.Strategy.Poll != nil:
		return reconcileResult{}, nil
	// Free cache and remove finalizer from object if sync strategy is "watch".
	case secretSync.Spec.Strategy.Watch != nil:
		if err := objecthandling.RemoveDynamicCacheLabel(ctx, r.client, &v1.Secret{
			TypeMeta: makeCoreV1SecretTypeMeta(),
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretSync.Spec.Src.Name,
				Namespace: secretSync.Spec.Src.Namespace,
			},
		}); client.IgnoreNotFound(err) != nil {
			return reconcileResult{}, fmt.Errorf("remove cache label from source secret: %w", err)
		}

		if err := objecthandling.FreeCacheAndRemoveFinalizer(ctx, r.client, secretSync, r.dynamicCache); err != nil {
			return reconcileResult{}, fmt.Errorf("free cache and remove finalizer: %w", err)
		}
		return reconcileResult{}, nil
	// Error out if sync strategy is neither of the above.
	default:
		return reconcileResult{}, fmt.Errorf("ENOTIMPLEMENTED: deleted secret sync does not employ a valid sync strategy.")
	}
}

var _ reconciler = (*secretReconciler)(nil)

type secretReconciler struct {
	client        client.Client
	ownerStrategy ownerStrategy
	dynamicCache  dynamicCache
}

func (r *secretReconciler) Reconcile(ctx context.Context, secretSync *v1alpha1.SecretSync) (reconcileResult, error) {
	// Do nothing if SecretSync is paused or is deleting.
	if secretSync.Spec.Paused || !secretSync.DeletionTimestamp.IsZero() {
		return reconcileResult{}, nil
	}

	// Take care of correctly caching and watching the source secret if strategy is "watch".
	if secretSync.Spec.Strategy.Watch != nil {
		// Add the cache finalizer to SecretSync object before dynamically caching the source Secret.
		if err := objecthandling.EnsureCachedFinalizer(ctx, r.client, secretSync); err != nil {
			return reconcileResult{}, fmt.Errorf("adding cached finalizer: %w", err)
		}

		// Ensure that the source Secret has the dynamic cache label.
		if err := objecthandling.EnsureDynamicCacheLabel(ctx, r.client, &v1.Secret{
			TypeMeta: makeCoreV1SecretTypeMeta(),
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretSync.Spec.Src.Name,
				Namespace: secretSync.Spec.Src.Namespace,
			},
		}); err != nil {
			return reconcileResult{}, fmt.Errorf("adding dynamic cache label: %w", err)
		}

		// Ensure that the SecretSync watches the source Secret for changes in our local cache.
		if err := r.dynamicCache.Watch(ctx, secretSync, &v1.Secret{
			TypeMeta: makeCoreV1SecretTypeMeta(),
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretSync.Spec.Src.Name,
				Namespace: secretSync.Spec.Src.Namespace,
			},
		}); err != nil {
			return reconcileResult{}, fmt.Errorf("watching source secret: %w", err)
		}
	}

	srcSecret := &v1.Secret{}
	if err := r.client.Get(ctx, types.NamespacedName{
		Namespace: secretSync.Spec.Src.Namespace,
		Name:      secretSync.Spec.Src.Name,
	}, srcSecret); err != nil {
		return reconcileResult{}, fmt.Errorf("getting source object: %w", err)
	}

	// Keep track of controlled objects.
	controllerOf := []v1alpha1.NamespacedName{}
	controllerOfLUT := map[v1alpha1.NamespacedName]struct{}{}

	// Sync to destination secrets.
	for _, dest := range secretSync.Spec.Dest {
		targetObject := srcSecret.DeepCopy()
		targetObject.ObjectMeta = metav1.ObjectMeta{
			Namespace: dest.Namespace,
			Name:      dest.Name,
			Labels: map[string]string{
				constants.DynamicCacheLabel: "True",
				managedByLabel:              secretSync.Name,
			},
		}

		if err := r.ownerStrategy.SetControllerReference(secretSync, targetObject); err != nil {
			return reconcileResult{}, fmt.Errorf("setting controller reference: %w", err)
		}

		if err := r.client.Patch(ctx, targetObject,
			client.Apply, client.ForceOwnership, client.FieldOwner(constants.FieldOwner)); err != nil {
			return reconcileResult{}, fmt.Errorf("patching destination secret: %w", err)
		}

		controllerOf = append(controllerOf, v1alpha1.NamespacedName{
			Namespace: dest.Namespace,
			Name:      dest.Name,
		})
		controllerOfLUT[v1alpha1.NamespacedName{
			Namespace: dest.Namespace,
			Name:      dest.Name,
		}] = struct{}{}
	}

	// Garbage collect secrets not managed by this SecretSync anymore.
	managedSecretsList := &v1.SecretList{}
	// managedSecretsList := &unstructured.UnstructuredList{
	// 	Object: map[string]interface{}{
	// 		"apiVersion": "v1",
	// 		"kind":       "SecretList",
	// 	},
	// }
	if err := r.dynamicCache.List(ctx, managedSecretsList, client.MatchingLabels{
		managedByLabel: secretSync.Name,
	}); err != nil {
		return reconcileResult{}, fmt.Errorf("listing managed secrets: %w", err)
	}

	fmt.Printf("managedSecretsList len(%d)\n", len(managedSecretsList.Items))

	// Delete secrets that are not managed anymore.
	for _, managedSecret := range managedSecretsList.Items {
		// Skip secrets that are still managed.
		if _, ok := controllerOfLUT[v1alpha1.NamespacedName{
			Namespace: managedSecret.Namespace,
			Name:      managedSecret.Name,
		}]; ok {
			continue
		}

		// Delete unmanaged secret, ignoring NotFound errors.
		if err := r.client.Delete(ctx, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: managedSecret.Namespace,
				Name:      managedSecret.Name,
			},
		}); client.IgnoreNotFound(err) != nil {
			return reconcileResult{}, fmt.Errorf("deleting uncontrolled Secret: %w", err)
		}
	}

	// Update Sync condition.
	condChanged := meta.SetStatusCondition(&secretSync.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.SecretSyncSync,
		Status:             metav1.ConditionTrue,
		Reason:             "SuccessfulSync",
		Message:            "Synchronization completed successfully.",
		ObservedGeneration: secretSync.Generation,
	})

	// Check if status would be changed before updating the rest of the status.
	statusChanged := condChanged ||
		!reflect.DeepEqual(secretSync.Status.ControllerOf, controllerOf) ||
		secretSync.Status.Phase != v1alpha1.SecretSyncStatusPhaseSync

	// Update rest of status.
	secretSync.Status.Phase = v1alpha1.SecretSyncStatusPhaseSync
	secretSync.Status.ControllerOf = controllerOf

	return reconcileResult{
		statusChanged: statusChanged,
	}, nil
}

var _ reconciler = (*pauseReconciler)(nil)

type pauseReconciler struct{}

func (r *pauseReconciler) Reconcile(_ context.Context, secretSync *v1alpha1.SecretSync) (reconcileResult, error) {
	condChanged := meta.SetStatusCondition(&secretSync.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.SecretSyncPaused,
		Status:             pausedBoolToConditionBool(secretSync.Spec.Paused),
		Reason:             pausedBoolToConditionReason(secretSync.Spec.Paused),
		ObservedGeneration: secretSync.Generation,
	})

	phaseIsWrong := secretSync.Spec.Paused && secretSync.Status.Phase != v1alpha1.SecretSyncStatusPhasePaused ||
		!secretSync.Spec.Paused && secretSync.Status.Phase != v1alpha1.SecretSyncStatusPhasePaused

	if phaseIsWrong && secretSync.Spec.Paused {
		secretSync.Status.Phase = v1alpha1.SecretSyncStatusPhasePaused
	} else if phaseIsWrong {
		secretSync.Status.Phase = v1alpha1.SecretSyncStatusPhaseSync
	}

	statusChanged := condChanged || phaseIsWrong

	return reconcileResult{
		statusChanged: statusChanged,
	}, nil
}

var _ reconciler = (*pollReconciler)(nil)

type pollReconciler struct{}

func (r *pollReconciler) Reconcile(_ context.Context, secretSync *v1alpha1.SecretSync) (reconcileResult, error) {
	// Skip requeueing for polling if SecretSync is paused or strategy is not "poll".
	if secretSync.Spec.Paused || secretSync.Spec.Strategy.Poll == nil {
		return reconcileResult{}, nil
	}

	return reconcileResult{
		res: reconcile.Result{
			RequeueAfter: secretSync.Spec.Strategy.Poll.Interval.Duration,
		},
	}, nil
}

func pausedBoolToConditionBool(b bool) metav1.ConditionStatus {
	if b {
		return metav1.ConditionTrue
	}

	return metav1.ConditionFalse
}

func pausedBoolToConditionReason(b bool) string {
	if b {
		return "SpecSaysPaused"
	}

	return "SpecSaysUnpaused"
}
