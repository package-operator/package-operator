package hostedclusterpackages

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
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

	"package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"

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

	hostedClusters := &v1beta1.HostedClusterList{}
	if err := c.client.List(ctx, hostedClusters, client.InNamespace("default")); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing clusters: %w", err)
	}

	for _, hc := range hostedClusters.Items {
		if err := c.reconcileHostedCluster(ctx, hostedClusterPackage, hc); err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling hosted cluster '%s': %w", hc.Name, err)
		}
	}

	return ctrl.Result{}, nil
}

func (c *HostedClusterPackageController) reconcileHostedCluster(
	ctx context.Context,
	clusterPackage *corev1alpha1.HostedClusterPackage,
	hc v1beta1.HostedCluster,
) error {
	log := logr.FromContextOrDiscard(ctx)

	if !meta.IsStatusConditionTrue(hc.Status.Conditions, v1beta1.HostedClusterAvailable) {
		log.Info(fmt.Sprintf("waiting for HostedCluster '%s' to become ready", hc.Name))
		return nil
	}

	pkg, err := c.constructClusterPackage(clusterPackage, hc)
	if err != nil {
		return fmt.Errorf("constructing Package: %w", err)
	}

	existingPkg := &corev1alpha1.Package{}
	err = c.client.Get(ctx, client.ObjectKeyFromObject(pkg), existingPkg)
	if err != nil && errors.IsNotFound(err) {
		if err := c.client.Create(ctx, pkg); err != nil {
			return fmt.Errorf("creating Package: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting Package: %w", err)
	}

	// Update package if spec is different.
	if !reflect.DeepEqual(existingPkg.Spec, pkg.Spec) {
		existingPkg.Spec = pkg.Spec
		if err := c.client.Update(ctx, existingPkg); err != nil {
			return fmt.Errorf("updating outdated Package: %w", err)
		}
	}

	return nil
}

func (c *HostedClusterPackageController) constructClusterPackage(
	clusterPackage *corev1alpha1.HostedClusterPackage,
	hc v1beta1.HostedCluster,
) (*corev1alpha1.Package, error) {
	pkg := &corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterPackage.Name,
			Namespace: v1beta1.HostedClusterNamespace(hc),
		},
		Spec: clusterPackage.Spec.PackageSpec,
	}

	if err := c.ownerStrategy.SetControllerReference(clusterPackage, pkg); err != nil {
		return nil, fmt.Errorf("setting controller reference: %w", err)
	}
	return pkg, nil
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
		Watches(
			&v1beta1.HostedCluster{},
			&handler.EnqueueRequestForObject{},
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
