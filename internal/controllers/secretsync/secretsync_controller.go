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

	secretSync := &v1alpha1.SecretSync{}
	if err := c.client.Get(ctx, req.NamespacedName, secretSync); err != nil {
		return ctrl.Result{}, fmt.Errorf("getting Secretsync: %w", err)
	}

	sourceObject := &v1.Secret{}

	if err := c.client.Get(ctx, types.NamespacedName{
		Namespace: secretSync.Spec.Src.Namespace,
		Name:      secretSync.Spec.Src.Name,
	}, sourceObject); err != nil {
		return ctrl.Result{}, fmt.Errorf("getting source object: %w", err)
	}

	// TODO: keep track of controlled objects
	controllerOf := []v1alpha1.NamespacedName{}
	controllerOfLUT := map[v1alpha1.NamespacedName]struct{}{}

	for _, dest := range secretSync.Spec.Dest {
		targetObject := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: dest.Namespace,
				Name:      dest.Name,
			},
			// wat? why do i have to specify this?
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			Immutable: sourceObject.Immutable,
			Type:      sourceObject.Type,
			Data:      sourceObject.Data,
		}

		if err := c.ownerStrategy.SetControllerReference(secretSync, targetObject); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting controller reference: %w", err)
		}

		// TODO: field owner string should be one central const over the whole project?
		// TODO: clarify client.ForceOwnership
		if err := c.client.Patch(ctx, targetObject,
			client.Apply, client.ForceOwnership, client.FieldOwner("package-operator")); err != nil {
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

	newStatus := *secretSync.Status.DeepCopy()
	newStatus.ControllerOf = controllerOf

	if !reflect.DeepEqual(secretSync.Status, newStatus) {
		// TODO: field owner string should be one central const over the whole project?
		// TODO: clarify client.ForceOwnership
		if err := c.client.Status().Patch(ctx, &v1alpha1.SecretSync{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretSync.Name,
				Namespace: secretSync.Namespace,
			},
			// TODO: Why do I have to add TypeMeta to prevent 'patching SecretSync status: invalid object type: /, Kind='
			TypeMeta: secretSync.TypeMeta,
			Status:   newStatus,
		},
			client.Apply, client.ForceOwnership, client.FieldOwner("package-operator")); err != nil {
			return ctrl.Result{}, fmt.Errorf("patching SecretSync status: %w", err)
		}
	}

	return ctrl.Result{}, nil
}
