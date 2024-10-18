package secretsync

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/objecthandling"
)

const ManagedByLabel = "package-operator.run/managed-by-secretsync"

var (
	ErrInvalidStrategy = errors.New("invalid strategy")
	ErrCollision       = errors.New("collision")
)

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

// Takes care of potential cleanup when object is deleting.
func (r *deletionReconciler) Reconcile(
	ctx context.Context, secretSync *corev1alpha1.SecretSync,
) (reconcileResult, error) {
	// Return early if object is not being deleted.
	if secretSync.DeletionTimestamp.IsZero() {
		return reconcileResult{}, nil
	}

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

		if err := objecthandling.FreeCacheAndRemoveFinalizer(
			ctx, r.client, secretSync, r.dynamicCache,
		); client.IgnoreNotFound(err) != nil {
			return reconcileResult{}, fmt.Errorf("free cache and remove finalizer: %w", err)
		}
		return reconcileResult{}, nil
	// Error out if sync strategy is neither of the above.
	default:
		return reconcileResult{}, fmt.Errorf("%w: strategy not implemented", ErrInvalidStrategy)
	}
}

var _ reconciler = (*secretReconciler)(nil)

type adoptionChecker interface {
	Check(owner, obj client.Object) (bool, error)
}

type secretReconciler struct {
	client          client.Client
	uncachedClient  client.Client
	ownerStrategy   ownerStrategy
	dynamicCache    dynamicCache
	adoptionChecker adoptionChecker
}

// srcReaderForStrategy returns the correct client for getting the source secret for the given strategy.
// Watch -> dynamicCache because the cache-label has been applied to the source and cache
// has been primed by calling .Watch()
// Poll -> uncachedClient - because the user explicitly opted out of caching/watching.
func (r *secretReconciler) srcReaderForStrategy(strategy corev1alpha1.SecretSyncStrategy) client.Reader {
	switch {
	case strategy.Watch != nil:
		return r.dynamicCache
	case strategy.Poll != nil:
		return r.uncachedClient
	default:
		panic(ErrInvalidStrategy)
	}
}

func (r *secretReconciler) Reconcile(
	ctx context.Context, secretSync *corev1alpha1.SecretSync,
) (reconcileResult, error) {
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
	if err := r.srcReaderForStrategy(secretSync.Spec.Strategy).Get(ctx, types.NamespacedName{
		Namespace: secretSync.Spec.Src.Namespace,
		Name:      secretSync.Spec.Src.Name,
	}, srcSecret); err != nil {
		return reconcileResult{}, fmt.Errorf("getting source object: %w", err)
	}

	// Keep track of controlled objects.
	controllerOf := []corev1alpha1.NamespacedName{}
	controllerOfLUT := map[corev1alpha1.NamespacedName]struct{}{}
	conflictIndices := []int{}

	// Sync to destination secrets.
	for index, dest := range secretSync.Spec.Dest {
		// Copy source secret while ensuring non-immutable destination secrets so
		// they can be updated later if the source is changed through re-creation.
		targetObject := srcSecret.DeepCopy()
		targetObject.Immutable = nil

		// Ensure correct typemeta for .Path() because even though
		// the dynamicCache doesn't strip it, the uncached client does.
		targetObject.TypeMeta = metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		}
		targetObject.ObjectMeta = metav1.ObjectMeta{
			Namespace: dest.Namespace,
			Name:      dest.Name,
			Labels: map[string]string{
				constants.DynamicCacheLabel: "True",
				ManagedByLabel:              secretSync.Name,
			},
		}

		// Try reconciling destination secret and record potentially encountered conflict.
		if err := r.reconcileSecret(ctx, secretSync, targetObject); errors.Is(err, ErrCollision) {
			log := logr.FromContextOrDiscard(ctx)
			log.Info("collision", "targetObject", targetObject)
			conflictIndices = append(conflictIndices, index)
			continue
		} else if err != nil {
			return reconcileResult{}, fmt.Errorf("reconciling secret: %w", err)
		}

		controllerOf = append(controllerOf, corev1alpha1.NamespacedName{
			Namespace: dest.Namespace,
			Name:      dest.Name,
		})
		controllerOfLUT[corev1alpha1.NamespacedName{
			Namespace: dest.Namespace,
			Name:      dest.Name,
		}] = struct{}{}
	}

	// Garbage collect secrets not managed by this SecretSync anymore.
	managedSecretsList := &v1.SecretList{}
	if err := r.dynamicCache.List(ctx, managedSecretsList, client.MatchingLabels{
		ManagedByLabel: secretSync.Name,
	}); err != nil {
		return reconcileResult{}, fmt.Errorf("listing managed secrets: %w", err)
	}

	// Delete secrets that are not managed anymore.
	for _, managedSecret := range managedSecretsList.Items {
		// Skip secrets that are still managed.
		if _, ok := controllerOfLUT[corev1alpha1.NamespacedName{
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
	syncCond := metav1.Condition{
		Type:               corev1alpha1.SecretSyncSync,
		ObservedGeneration: secretSync.Generation,
	}
	if len(conflictIndices) > 0 {
		syncCond.Status = metav1.ConditionFalse
		syncCond.Reason = "EncounteredAtLeastOneConflict"
		syncCond.Message = "Indices in .spec.dest[] with conflicts: " + intSliceToCSV(conflictIndices)
	} else {
		syncCond.Status = metav1.ConditionTrue
		syncCond.Reason = "SuccessfulSync"
		syncCond.Message = "Synchronization completed successfully."
	}
	condChanged := meta.SetStatusCondition(&secretSync.Status.Conditions, syncCond)

	// Check if status would be changed before updating the rest of the status.
	statusChanged := condChanged ||
		!reflect.DeepEqual(secretSync.Status.ControllerOf, controllerOf) ||
		secretSync.Status.Phase != corev1alpha1.SecretSyncStatusPhaseSync

	// Update rest of status.
	secretSync.Status.Phase = corev1alpha1.SecretSyncStatusPhaseSync
	secretSync.Status.ControllerOf = controllerOf

	return reconcileResult{
		statusChanged: statusChanged,
	}, nil
}

func (r *secretReconciler) reconcileSecret(
	ctx context.Context, owner *corev1alpha1.SecretSync,
	targetObject *v1.Secret,
) error {
	if err := r.ownerStrategy.SetControllerReference(owner, targetObject); err != nil {
		return fmt.Errorf("setting controller reference: %w", err)
	}

	// Ensure to watch Secrets.
	if err := r.dynamicCache.Watch(
		ctx, owner, targetObject); err != nil {
		return fmt.Errorf("watching new resource: %w", err)
	}

	// Check if destination secret already exists.
	currentObject := targetObject.DeepCopy()
	objectKey := client.ObjectKeyFromObject(currentObject)

	// Try cached lookup first.
	err := r.dynamicCache.Get(
		ctx,
		objectKey,
		currentObject,
	)
	if err != nil && !apimachineryerrors.IsNotFound(err) {
		return fmt.Errorf("getting destination secret: %w", err)
	}
	if apimachineryerrors.IsNotFound(err) {
		// Do an uncached lookup if not found in cache.
		err = r.uncachedClient.Get(ctx, objectKey, currentObject)
	}
	if err != nil && !apimachineryerrors.IsNotFound(err) {
		return fmt.Errorf("getting destination secret (uncached): %w", err)
	}
	if apimachineryerrors.IsNotFound(err) {
		// The object is not yet present on the cluster, just create it using desired state!
		err := r.client.Patch(ctx, targetObject, client.Apply, client.ForceOwnership, client.FieldOwner(constants.FieldOwner))
		switch {
		case apimachineryerrors.IsAlreadyExists(err):
			// Now the object already exists, but was neither in our cache, nor in the cluster before.
			// Get object via uncached client directly from the API server and fall through to update code below.
			if err := r.uncachedClient.Get(ctx, objectKey, currentObject); err != nil {
				return fmt.Errorf("getting destination secret (uncached/alreadyExisted): %w", err)
			}
		case err != nil:
			return fmt.Errorf("patching destination secret: %w", err)
		default:
			return nil
		}
	}

	// Destination secret already exists.
	// Check if it needs adoption and only update if we either already
	// are or can become the controller of this object.
	updatedObject := currentObject.DeepCopy()

	needsAdoption, err := r.adoptionChecker.Check(owner, currentObject)
	if err != nil {
		return fmt.Errorf("checking adoption needs for destination secret: %w", err)
	}

	if needsAdoption {
		log := logr.FromContextOrDiscard(ctx)
		log.Info("adopting secret",
			"OwnerKey", client.ObjectKeyFromObject(owner),
			"ObjectKey", client.ObjectKeyFromObject(targetObject))
		r.ownerStrategy.ReleaseController(updatedObject)
		if err := r.ownerStrategy.SetControllerReference(owner, updatedObject); err != nil {
			return fmt.Errorf("setting controller reference: %w", err)
		}
	}

	if !r.ownerStrategy.IsController(owner, updatedObject) {
		return fmt.Errorf("%w: already exists and not controlled by us", ErrCollision)
	}

	if err := r.client.Patch(
		ctx, targetObject, client.Apply, client.ForceOwnership,
		client.FieldOwner(constants.FieldOwner),
	); err != nil {
		return fmt.Errorf("patching destination secret: %w", err)
	}
	return nil
}

var _ reconciler = (*pauseReconciler)(nil)

type pauseReconciler struct{}

func (r *pauseReconciler) Reconcile(_ context.Context, secretSync *corev1alpha1.SecretSync) (reconcileResult, error) {
	if !secretSync.DeletionTimestamp.IsZero() {
		return reconcileResult{}, nil
	}

	condChanged := meta.SetStatusCondition(&secretSync.Status.Conditions, metav1.Condition{
		Type:               corev1alpha1.SecretSyncPaused,
		Status:             pausedBoolToConditionBool(secretSync.Spec.Paused),
		Reason:             pausedBoolToConditionReason(secretSync.Spec.Paused),
		ObservedGeneration: secretSync.Generation,
	})

	phaseIsWrong := secretSync.Spec.Paused && secretSync.Status.Phase != corev1alpha1.SecretSyncStatusPhasePaused ||
		!secretSync.Spec.Paused && secretSync.Status.Phase != corev1alpha1.SecretSyncStatusPhasePaused

	if phaseIsWrong && secretSync.Spec.Paused {
		secretSync.Status.Phase = corev1alpha1.SecretSyncStatusPhasePaused
	} else if phaseIsWrong {
		secretSync.Status.Phase = corev1alpha1.SecretSyncStatusPhaseSync
	}

	statusChanged := condChanged || phaseIsWrong

	return reconcileResult{
		statusChanged: statusChanged,
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

func intSliceToCSV(slice []int) string {
	s := []string{}
	for _, i := range slice {
		s = append(s, strconv.Itoa(i))
	}
	return strings.Join(s, ", ")
}
