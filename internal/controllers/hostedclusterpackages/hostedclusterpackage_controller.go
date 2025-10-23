package hostedclusterpackages

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"pkg.package-operator.run/boxcutter/ownerhandling"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/constants"
)

type HostedClusterPackageController struct {
	client        client.Client
	log           logr.Logger
	scheme        *runtime.Scheme
	ownerStrategy ownerStrategy
}

type ownerStrategy interface {
	SetControllerReference(owner, obj metav1.Object) error
	EnqueueRequestForOwner(
		ownerType client.Object, mapper meta.RESTMapper, isController bool,
	) handler.EventHandler
}

func NewHostedClusterPackageController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
) *HostedClusterPackageController {
	return &HostedClusterPackageController{
		client: c,
		log:    log,
		scheme: scheme,
		// Using Annotation Owner-Handling,
		// because Package objects will live in the hosted-clusters "execution" namespace.
		// e.g. clusters-my-cluster and not in the same Namespace as the HostedCluster object
		ownerStrategy: ownerhandling.NewAnnotation(scheme, constants.OwnerStrategyAnnotationKey),
	}
}

func (c *HostedClusterPackageController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("HostedClusterPackage", req.String())
	defer log.Info("reconciled")

	ctx = logr.NewContext(ctx, log)
	hostedClusterPackage := &corev1alpha1.HostedClusterPackage{}
	if err := c.client.Get(ctx, req.NamespacedName, hostedClusterPackage); err != nil {
		// Ignore not found errors on delete
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !hostedClusterPackage.DeletionTimestamp.IsZero() {
		log.Info("HostedClusterPackage is deleting")
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (c *HostedClusterPackageController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.HostedClusterPackage{}).
		WatchesRawSource(
			source.Kind(
				mgr.GetCache(),
				&corev1alpha1.Package{},
				wrapEventHandlerwithTypedEventHandler[*corev1alpha1.Package](
					c.ownerStrategy.EnqueueRequestForOwner(&corev1alpha1.HostedClusterPackage{},
						mgr.GetRESTMapper(),
						true,
					),
				),
			),
		).
		Complete(c)
}

type outer[T client.Object] struct {
	inner handler.TypedEventHandler[client.Object, reconcile.Request]
}

// Create implements handler.TypedEventHandler.
func (o outer[T]) Create(ctx context.Context, evt event.TypedCreateEvent[T],
	rl workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	o.inner.Create(ctx, event.TypedCreateEvent[client.Object]{Object: evt.Object}, rl)
}

// Delete implements handler.TypedEventHandler.
func (o outer[T]) Delete(ctx context.Context, evt event.TypedDeleteEvent[T],
	rl workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	o.inner.Delete(
		ctx,
		event.TypedDeleteEvent[client.Object]{Object: evt.Object, DeleteStateUnknown: evt.DeleteStateUnknown},
		rl,
	)
}

// Generic implements handler.TypedEventHandler.
func (o outer[T]) Generic(ctx context.Context, evt event.TypedGenericEvent[T],
	rl workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	o.inner.Generic(ctx, event.TypedGenericEvent[client.Object]{Object: evt.Object}, rl)
}

// Update implements handler.TypedEventHandler.
func (o outer[T]) Update(ctx context.Context, evt event.TypedUpdateEvent[T],
	rl workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	o.inner.Update(ctx, event.TypedUpdateEvent[client.Object]{ObjectOld: evt.ObjectOld, ObjectNew: evt.ObjectNew}, rl)
}

func wrapEventHandlerwithTypedEventHandler[T client.Object](
	inner handler.TypedEventHandler[client.Object, reconcile.Request],
) handler.TypedEventHandler[T, reconcile.Request] {
	return outer[T]{inner}
}
