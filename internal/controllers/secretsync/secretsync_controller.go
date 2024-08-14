package secretsync

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
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
	"package-operator.run/internal/dynamiccachehandling"
	"package-operator.run/internal/ownerhandling"
)

type Controller struct {
	log    logr.Logger
	client client.Client
	scheme *runtime.Scheme

	dynamicCache dynamicCache

	ownerStrategy ownerStrategy
}

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
		return ctrl.Result{}, fmt.Errorf("getting Secretsync: %w", err)
	}

	// Do nothing if object is deleting and sync strategy is "poll".
	if !secretSync.DeletionTimestamp.IsZero() && secretSync.Spec.Strategy.Poll != nil {
		return ctrl.Result{}, nil
	}

	// Get source Secret.
	srcSecret := &v1.Secret{}
	if err := c.client.Get(ctx, types.NamespacedName{
		Namespace: secretSync.Spec.Src.Namespace,
		Name:      secretSync.Spec.Src.Name,
	}, srcSecret); err != nil {
		return ctrl.Result{}, fmt.Errorf("getting source object: %w", err)
	}

	// TODO: The cache label removal on srcSecret should be guarded by a finalizer, right?

	// Do nothing except releasing the srcSecret from our syncStrategy if object is deleting.
	if !secretSync.DeletionTimestamp.IsZero() {
		// Refactoring / API-Extension guard: Panic if we got here but the strategy is not "watch".
		// This should only ever happen if a new strategy was introduced and the implementation
		// of this controller wasn't changed to reflect that or the code was refactored.
		if secretSync.Spec.Strategy.Watch == nil {
			panic(
				fmt.Errorf("ENOTIMPLEMENTED: deleted secret sync not employ .spec.strategy.poll even though it is expected in the code: %s",
					secretSync.Namespace,
					secretSync.Name,
				),
			)
		}

		if err := dynamiccachehandling.RemoveDynamicCacheLabel(ctx, c.client, secretSync); err != nil {
			return ctrl.Result{}, fmt.Errorf("removing dynamic cache label from source secret: %w")
		}

		return ctrl.Result{}, nil
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
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("deleting uncontrolled Secret: %w", err)
		}
	}

	// Update status.
	// TODO change from .Path to .Update
	newStatus := *secretSync.Status.DeepCopy()
	newStatus.ControllerOf = controllerOf
	if !reflect.DeepEqual(secretSync.Status, newStatus) {

		if err := c.client.Status().Patch(ctx, &v1alpha1.SecretSync{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretSync.Name,
				Namespace: secretSync.Namespace,
			},
			TypeMeta: secretSync.TypeMeta,
			Status:   newStatus,
		},
			client.Apply, client.ForceOwnership, client.FieldOwner(controllers.FieldOwner)); err != nil {
			return ctrl.Result{}, fmt.Errorf("patching SecretSync status: %w", err)
		}
	}

	// Let syncStrategy decide if reconciliation request should be requeued after a certain time.
	res := &ctrl.Result{}
	c.syncStrategy.SetRequeueAfter(res)
	return *res, nil
}
