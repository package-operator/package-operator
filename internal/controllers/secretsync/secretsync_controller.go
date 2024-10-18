package secretsync

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/dynamiccache"
	"package-operator.run/internal/ownerhandling"
)

type dynamicCache interface {
	client.Reader
	Source(handler handler.EventHandler, predicates ...predicate.Predicate) source.Source
	Free(ctx context.Context, obj client.Object) error
	Watch(ctx context.Context, owner client.Object, obj runtime.Object) error
	OwnersForGKV(gvk schema.GroupVersionKind) []dynamiccache.OwnerReference
}

type ownerStrategy interface {
	IsController(owner, obj metav1.Object) bool
	HasController(obj metav1.Object) bool
	ReleaseController(obj metav1.Object)
	SetControllerReference(owner, obj metav1.Object) error
}

type reconcileResult struct {
	statusChanged bool
}

type reconciler interface {
	Reconcile(ctx context.Context, req *corev1alpha1.SecretSync) (reconcileResult, error)
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
	uncachedClient client.Client,
) *Controller {
	return &Controller{
		log:           log,
		client:        client,
		scheme:        scheme,
		dynamicCache:  dynamicCache,
		ownerStrategy: ownerhandling.NewNative(scheme),
		reconcilers: []reconciler{
			&deletionReconciler{
				client:       client,
				dynamicCache: dynamicCache,
			},
			&secretReconciler{
				client:        client,
				ownerStrategy: ownerhandling.NewNative(scheme),
				adoptionChecker: &defaultAdoptionChecker{
					ownerStrategy: ownerhandling.NewNative(scheme),
				},
				dynamicCache:   dynamicCache,
				uncachedClient: uncachedClient,
			},
			&pauseReconciler{},
		},
	}
}

func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.SecretSync{}).
		WatchesRawSource(
			c.dynamicCache.Source(
				handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &corev1alpha1.SecretSync{}),
				predicate.NewPredicateFuncs(func(object client.Object) bool {
					c.log.Info(
						"processing dynamic cache event",
						"object", client.ObjectKeyFromObject(object),
						"owners", object.GetOwnerReferences(),
						"gvk", object.GetObjectKind().GroupVersionKind(),
					)
					return true
				}),
			),
		).
		WatchesRawSource(
			c.dynamicCache.Source(
				dynamiccache.NewEnqueueWatchingObjects(c.dynamicCache, &corev1alpha1.SecretSync{}, mgr.GetScheme()),
				predicate.NewPredicateFuncs(func(object client.Object) bool {
					c.log.Info(
						"processing dynamic cache event",
						"gvk", object.GetObjectKind().GroupVersionKind(),
						"object", client.ObjectKeyFromObject(object),
						"owners", object.GetOwnerReferences(),
					)
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
	secretSync := &corev1alpha1.SecretSync{}
	if err := c.client.Get(ctx, req.NamespacedName, secretSync); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(fmt.Errorf("getting Secretsync: %w", err))
	}

	var (
		statusChanged bool
		err           error
	)
	for _, reconciler := range c.reconcilers {
		rr, errI := reconciler.Reconcile(ctx, secretSync)
		if rr.statusChanged {
			statusChanged = true
		}
		if errI != nil {
			err = errI
			break
		}
	}

	if statusChanged {
		errS := c.client.Status().Update(ctx, secretSync)
		if errS != nil {
			err = errors.Join(err, fmt.Errorf("updating SecretSync status: %w", errS))
		}
	}

	// Skip requeueing for polling if SecretSync is paused or strategy is not "poll".
	if secretSync.Spec.Paused || secretSync.Spec.Strategy.Poll == nil {
		return ctrl.Result{}, err
	}

	// Requeue for polling strategy.
	return ctrl.Result{
		RequeueAfter: secretSync.Spec.Strategy.Poll.Interval.Duration,
	}, err
}
