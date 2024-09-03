package secretsync

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/objecthandling"
	"package-operator.run/internal/ownerhandling"
)

type dynamicCache interface {
	client.Reader
	Source(handler handler.EventHandler, predicates ...predicate.Predicate) source.Source
	Free(ctx context.Context, obj client.Object) error
	Watch(ctx context.Context, owner client.Object, obj runtime.Object) error
}

type ownerStrategy interface {
	ReleaseController(obj metav1.Object)
	SetControllerReference(owner, obj metav1.Object) error
}

type reconcileResult struct {
	res           ctrl.Result
	statusChanged bool
}

type reconciler interface {
	Reconcile(ctx context.Context, req *v1alpha1.SecretSync) (reconcileResult, error)
}

type Controller struct {
	log           logr.Logger
	client        client.Client
	scheme        *runtime.Scheme
	dynamicCache  dynamicCache
	ownerStrategy ownerStrategy
	reconcilers   []reconciler
}

func NewController(
	client client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	dynamicCache dynamicCache,
) *Controller {
	return &Controller{
		log:           log,
		client:        client,
		scheme:        scheme,
		dynamicCache:  dynamicCache,
		ownerStrategy: ownerhandling.NewNative(scheme),
	}
}

func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.SecretSync{}).
		WatchesRawSource(
			c.dynamicCache.Source(
				handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &v1alpha1.SecretSync{}),
				predicate.NewPredicateFuncs(func(object client.Object) bool {
					c.log.Info(
						"processing dynamic cache event",
						"object", client.ObjectKeyFromObject(object),
						"owners", object.GetOwnerReferences())
					return true
				}),
			),
		).
		Complete(c)
}

func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := c.log.WithValues("SecretSync", req.String())
	defer log.Info("reconciled")

	// Get SecretSync from cluster.
	secretSync := &v1alpha1.SecretSync{}
	if err := c.client.Get(ctx, req.NamespacedName, secretSync); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(fmt.Errorf("getting Secretsync: %w", err))
	}

	var (
		statusChanged bool
		res           ctrl.Result
		err           error
	)
	for _, reconciler := range c.reconcilers {
		rr, errI := reconciler.Reconcile(ctx, secretSync)
		if rr.statusChanged {
			statusChanged = true
		}
		res = rr.res
		if errI != nil {
			err = errI
			break
		}
	}

	if statusChanged {
		if err := c.client.Status().Update(ctx, secretSync); err != nil {
			return res, fmt.Errorf("updating SecretSync status: %w", err)
		}
	}

	return res, err

	// reconcililiations / phases should be

	// - ensure cache finalizer presence/absence (and free caches if needed)
	// - return early if in deletion
	// - establish conditions
	// - reconcile pause condition + phase
	// -

	pauseCondChanged := meta.SetStatusCondition(&secretSync.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.SecretSyncPaused,
		Status:             pausedBoolToConditionBool(secretSync.Spec.Paused),
		Reason:             pausedBoolToConditionReason(secretSync.Spec.Paused),
		ObservedGeneration: secretSync.Generation,
	})

	// If SecretSync is paused and phase and paused condition are already correct and fresh; Return early.
	if secretSync.Spec.Paused && secretSync.Status.Phase == v1alpha1.SecretSyncStatusPhasePaused &&
		!pauseCondChanged {
		return ctrl.Result{}, nil
	}

	// erdii: Don't rewrite this into a single boolean expression. No one will be able to understand it.
	{
		// If SecretSync is not paused, but phase or paused condition still say that it is paused: update status.
		updatePauseStatus := false
		if !secretSync.Spec.Paused &&
			(secretSync.Status.Phase == v1alpha1.SecretSyncStatusPhasePaused || pauseCondChanged) {
			updatePauseStatus = true
		}
		// Or if SecretSync is paused, but phase or paused condition don't match
		// (otherwise code would have returned early above): update status.
		if secretSync.Spec.Paused {
			updatePauseStatus = true
		}

		if updatePauseStatus {
			// Update status.
			// Set correct phase.
			secretSync.Status.Phase = pausedBoolToPhase(secretSync.Spec.Paused)
			intentObject := &v1alpha1.SecretSync{
				TypeMeta: secretSync.TypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name: secretSync.Name,
				},
				Status: secretSync.Status,
			}

			if err := c.client.Status().Patch(
				ctx,
				intentObject,
				client.Apply,
				client.FieldOwner(controllers.FieldOwner),
			); err != nil {
				return ctrl.Result{}, fmt.Errorf("updating paused SecretSync status: %w", err)
			}
			// Copy changed resource version, in case something wants to .Update the SecretSync object later.
			// (Looking at you EnsureCachedFinalizer() 🤨)
			secretSync.ResourceVersion = intentObject.ResourceVersion
		}
	}

	// If Paused - do nothing except communicating pause status.
	if secretSync.Spec.Paused {
		return ctrl.Result{}, nil
	}

	// Do nothing if object is deleting and sync strategy is "poll".
	if !secretSync.DeletionTimestamp.IsZero() && secretSync.Spec.Strategy.Poll != nil {
		return ctrl.Result{}, nil
	}

	// Ensure cache finalizer if sync strategy is "watch".
	if secretSync.Spec.Strategy.Watch != nil && secretSync.DeletionTimestamp.IsZero() {
		if err := objecthandling.EnsureCachedFinalizer(ctx, c.client, secretSync); err != nil {
			return ctrl.Result{}, fmt.Errorf("ensuring cached finalizer: %w", err)
		}
	}

	// Get source Secret.
	srcSecret := &v1.Secret{}
	if err := c.client.Get(ctx, types.NamespacedName{
		Namespace: secretSync.Spec.Src.Namespace,
		Name:      secretSync.Spec.Src.Name,
	}, srcSecret); err != nil {
		return ctrl.Result{}, fmt.Errorf("getting source object: %w", err)
	}

	// Do nothing except releasing the srcSecret from our ownerCache if object is deleting.
	if !secretSync.DeletionTimestamp.IsZero() {
		// Refactoring / API-Extension guard: Panic if we got here but the strategy is not "watch".
		// This should only ever happen if a new strategy was introduced and the implementation
		// of this controller wasn't changed to reflect that or the code was refactored.
		if secretSync.Spec.Strategy.Watch == nil {
			panic(
				"ENOTIMPLEMENTED: deleted secret sync does not employ .spec.strategy.poll " +
					"even though it is expected in the code",
			)
		}

		if err := objecthandling.FreeCacheAndRemoveFinalizer(ctx, c.client, secretSync, c.dynamicCache); err != nil {
			return ctrl.Result{}, fmt.Errorf("free cache and remove finalizer: %w", err)
		}

		// Free cache from srcSecret.
		if err := c.dynamicCache.Free(ctx, srcSecret); err != nil {
			return ctrl.Result{}, fmt.Errorf("cache freeing src secret: %w", err)
		}

		return ctrl.Result{}, nil
	}

	// Take care of dynamic caching if strategy is "watch".
	if secretSync.Spec.Strategy.Watch != nil {
		// Ensure that the source Secret has the dynamic cache label.
		if !objecthandling.HasDynamicCacheLabel(srcSecret) {
			if err := objecthandling.EnsureDynamicCacheLabel(ctx, c.client, srcSecret); err != nil {
				return ctrl.Result{}, fmt.Errorf("adding dynamic cache label: %w", err)
			}
		}
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
				controllers.DynamicCacheLabel: "True",
			},
		}

		if err := c.ownerStrategy.SetControllerReference(secretSync, targetObject); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting controller reference: %w", err)
		}

		if err := c.client.Patch(ctx, targetObject,
			client.Apply, client.ForceOwnership, client.FieldOwner(controllers.FieldOwner)); err != nil {
			return ctrl.Result{}, fmt.Errorf("patching destination secret: %w", err)
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

	// Garbage collect Secrets which aren't controlled by this SecretSync anymore.
	for _, controlledSecretRef := range secretSync.Status.ControllerOf {
		if _, ok := controllerOfLUT[controlledSecretRef]; ok {
			continue
		}

		if err := c.client.Delete(ctx, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: controlledSecretRef.Namespace,
				Name:      controlledSecretRef.Name,
			},
		}); err != nil && !apimachineryerrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("deleting uncontrolled Secret: %w", err)
		}
	}

	// Update status if it changed.
	newStatus := *secretSync.Status.DeepCopy()
	newStatus.ControllerOf = controllerOf
	newStatus.Phase = v1alpha1.SecretSyncStatusPhaseSync
	meta.SetStatusCondition(&newStatus.Conditions, metav1.Condition{
		Type:               v1alpha1.SecretSyncSync,
		Status:             metav1.ConditionTrue,
		Reason:             "SuccessfulSync",
		Message:            "Synchronization completed successfully.",
		ObservedGeneration: secretSync.Generation,
	})
	if !reflect.DeepEqual(secretSync.Status, newStatus) {
		secretSync.Status = newStatus
		// TODO: why am I not allowed to disable optimistic locking here?
		// secretSync.ObjectMeta.ResourceVersion = ""
		// gives me this error:
		//nolint:lll
		// 2024-08-28T10:18:46+02:00       ERROR   Reconciler error        {"controller": "secretsync", "controllerGroup": "package-operator.run", "controllerKind": "SecretSync", "SecretSync": {"name":"bootstrap-token"}, "namespace": "", "name": "bootstrap-token", "reconcileID": "5603ba3a-a5c7-4167-bc9b-03e8e0bc17f0", "error": "updating SecretSync status: secretsyncs.package-operator.run \"bootstrap-token\" is invalid: metadata.resourceVersion: Invalid value: 0x0: must be specified for an update"}
		// metadata.resourceVersion: Invalid value: 0x0: must be specified for an update
		// if err := c.client.Status().Update(ctx, secretSync, client.FieldOwner(controllers.FieldOwner)); err != nil {
		// 	return ctrl.Result{}, fmt.Errorf("updating SecretSync status: %w", err)
		// }

		// Using SSA instead works:
		intentObject := &v1alpha1.SecretSync{
			TypeMeta: secretSync.TypeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name: secretSync.Name,
			},
			Status: secretSync.Status,
		}
		if err := c.client.Status().Patch(
			ctx,
			intentObject,
			client.Apply,
			client.FieldOwner(controllers.FieldOwner),
		); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating SecretSync status: %w", err)
		}
		// Copy changed resource version over, in case something wants to .Update the SecretSync object later.
		secretSync.ResourceVersion = intentObject.ResourceVersion
	}

	// If sync strategy "poll": Requeue after poll interval.
	if secretSync.Spec.Strategy.Poll != nil {
		return ctrl.Result{
			RequeueAfter: secretSync.Spec.Strategy.Poll.Interval.Duration,
		}, nil
	}

	return ctrl.Result{}, nil
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

func pausedBoolToPhase(b bool) v1alpha1.SecretSyncStatusPhase {
	if b {
		return v1alpha1.SecretSyncStatusPhasePaused
	}

	return v1alpha1.SecretSyncStatusPhasePending
}
