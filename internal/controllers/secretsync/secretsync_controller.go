package secretsync

import (
	"context"
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

	"package-operator.run/apis/core/v1alpha1"
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
		reconcilers: []reconciler{
			&deletionReconciler{
				client:       client,
				dynamicCache: dynamicCache,
			},
			&secretReconciler{
				client:        client,
				ownerStrategy: ownerhandling.NewNative(scheme),
				dynamicCache:  dynamicCache,
			},
			&pauseReconciler{},
			&pollReconciler{},
		},
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
		WatchesRawSource(
			c.dynamicCache.Source(
				dynamiccache.NewEnqueueWatchingObjects(c.dynamicCache, &v1alpha1.SecretSync{}, mgr.GetScheme()),
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
		if !rr.res.IsZero() {
			res = rr.res
			break
		}
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
}
