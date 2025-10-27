package hostedclusterpackages

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"pkg.package-operator.run/boxcutter/ownerhandling"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/constants"
)

const (
	packageNameIndexKey      = "metadata.name"
	minReadyDuration         = 10 * time.Second
	cleanupPackagesFinalizer = "package-operator.run/cleanup-packages"
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
		// TODO: detect type of object causing reconcile?

		// Ignore not found errors on delete
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !hostedClusterPackage.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, c.handleDeletion(ctx, hostedClusterPackage)
	}

	if !controllerutil.ContainsFinalizer(hostedClusterPackage, cleanupPackagesFinalizer) {
		controllerutil.AddFinalizer(hostedClusterPackage, cleanupPackagesFinalizer)
		if err := c.client.Update(ctx, hostedClusterPackage); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding cleanup-packages finalizer: %w", err)
		}
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

	return c.updateStatus(ctx, hostedClusterPackage, hostedClusters)
}

func (c *HostedClusterPackageController) handleDeletion(
	ctx context.Context,
	hostedClusterPackage *corev1alpha1.HostedClusterPackage,
) error {
	if !controllerutil.ContainsFinalizer(hostedClusterPackage, cleanupPackagesFinalizer) {
		return nil
	}

	log := logr.FromContextOrDiscard(ctx)
	log.Info("HostedClusterPackage is deleting")

	packages := &corev1alpha1.PackageList{}
	if err := c.client.List(ctx, packages, client.MatchingFields{
		packageNameIndexKey: hostedClusterPackage.Name,
	}); err != nil {
		return fmt.Errorf("listing packages: %w", err)
	}
	for _, pkg := range packages.Items {
		if err := c.client.Delete(ctx, &pkg); err != nil {
			return fmt.Errorf("deleting package in '%s': %w", pkg.Namespace, err)
		}
	}

	controllerutil.RemoveFinalizer(hostedClusterPackage, cleanupPackagesFinalizer)
	if err := c.client.Update(ctx, hostedClusterPackage); err != nil {
		return fmt.Errorf("removing cleanup-packages finalizer: %w", err)
	}
	return nil
}

func (c *HostedClusterPackageController) updateStatus(
	ctx context.Context,
	hostedClusterPackage *corev1alpha1.HostedClusterPackage,
	hostedClusters *v1beta1.HostedClusterList,
) (ctrl.Result, error) {
	packages := &corev1alpha1.PackageList{}
	if err := c.client.List(ctx, packages, client.MatchingFields{
		packageNameIndexKey: hostedClusterPackage.Name,
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing packages: %w", err)
	}

	hostedClusterPackage.Status.Packages = int32(len(packages.Items))
	hostedClusterPackage.Status.AvailablePackages = 0
	hostedClusterPackage.Status.ReadyPackages = 0
	hostedClusterPackage.Status.UpdatedPackages = 0

	requeueAfter := 2 * minReadyDuration
	for _, pkg := range packages.Items {
		// TODO: invalid package image ref -> package is "Available" && !"Unpacked"?

		availableCond := meta.FindStatusCondition(pkg.Status.Conditions, corev1alpha1.PackageAvailable)
		if availableCond != nil && availableCond.Status == metav1.ConditionTrue {
			readyFor := time.Now().UTC().Sub(availableCond.LastTransitionTime.Time)
			if readyFor >= minReadyDuration {
				hostedClusterPackage.Status.AvailablePackages++
			} else {
				requeueAfter = min(requeueAfter, minReadyDuration-readyFor+time.Second)
			}
		}

		if meta.IsStatusConditionFalse(pkg.Status.Conditions, corev1alpha1.PackageProgressing) {
			hostedClusterPackage.Status.ReadyPackages++
		}

		if reflect.DeepEqual(pkg.Spec, hostedClusterPackage.Spec.PackageSpec) {
			hostedClusterPackage.Status.UpdatedPackages++
		}
	}

	hostedClusterPackage.Status.UnavailablePackages = int32(len(hostedClusters.Items)) -
		hostedClusterPackage.Status.AvailablePackages

	c.updateConditions(hostedClusterPackage)

	if err := c.client.Status().Update(ctx, hostedClusterPackage); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	if requeueAfter < 2*minReadyDuration {
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	return ctrl.Result{}, nil
}

func (c *HostedClusterPackageController) updateConditions(hostedClusterPackage *corev1alpha1.HostedClusterPackage) {
	available := metav1.ConditionTrue
	progressing := metav1.ConditionTrue
	if hostedClusterPackage.Status.UnavailablePackages == 0 {
		progressing = metav1.ConditionFalse
	} else {
		available = metav1.ConditionFalse
	}

	meta.SetStatusCondition(&hostedClusterPackage.Status.Conditions, metav1.Condition{
		Type:               corev1alpha1.HostedClusterPackageAvailable,
		Status:             available,
		ObservedGeneration: hostedClusterPackage.Generation,
		Reason:             "TODO:Reason",
	})
	meta.SetStatusCondition(&hostedClusterPackage.Status.Conditions, metav1.Condition{
		Type:               corev1alpha1.HostedClusterPackageProgressing,
		Status:             progressing,
		ObservedGeneration: hostedClusterPackage.Generation,
		Reason:             "TODO:Reason",
	})
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
	if errors.IsNotFound(err) {
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
	// Index Packages by name
	if err := mgr.GetCache().IndexField(context.Background(), &corev1alpha1.Package{}, packageNameIndexKey,
		func(obj client.Object) []string {
			return []string{obj.GetName()}
		}); err != nil {
		return fmt.Errorf("creating name index for Packages: %w", err)
	}

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
