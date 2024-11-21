package objectsetphases

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"pkg.package-operator.run/boxcutter/machinery"
	"pkg.package-operator.run/boxcutter/machinery/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"pkg.package-operator.run/boxcutter/machinery/ownerhandling"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/controllers"
)

type reconciler interface {
	Reconcile(ctx context.Context, objectSet genericObjectSetPhase) (ctrl.Result, error)
}

type dynamicCache interface {
	client.Reader
	Source(handler handler.EventHandler, predicates ...predicate.Predicate) source.Source
	Free(ctx context.Context, obj client.Object) error
	Watch(ctx context.Context, owner client.Object, obj runtime.Object) error
}

type ownerStrategy interface {
	GetController(obj metav1.Object) (metav1.OwnerReference, bool)
	IsController(owner, obj metav1.Object) bool
	IsOwner(owner, obj metav1.Object) bool
	ReleaseController(obj metav1.Object)
	RemoveOwner(owner, obj metav1.Object)
	SetOwnerReference(owner, obj metav1.Object) error
	SetControllerReference(owner, obj metav1.Object) error
	EnqueueRequestForOwner(
		ownerType client.Object, mapper meta.RESTMapper, isController bool,
	) handler.EventHandler
	CopyOwnerReferences(objA, objB metav1.Object)
}

type teardownHandler interface {
	Teardown(
		ctx context.Context, objectSetPhase genericObjectSetPhase,
	) (cleanupDone bool, err error)
}

type phaseValidator interface {
	Validate(
		ctx context.Context,
		owner client.Object,
		phase validation.Phase,
	) (validation.PhaseViolation, error)
}

// Generic reconciler for both ObjectSetPhase and ClusterObjectSetPhase objects.
type GenericObjectSetPhaseController struct {
	newObjectSetPhase genericObjectSetPhaseFactory

	class           string // Phase class this controller is operating for.
	log             logr.Logger
	scheme          *runtime.Scheme
	client          client.Client // client to get and update ObjectSetPhases.
	dynamicCache    dynamicCache
	ownerStrategy   ownerStrategy
	teardownHandler teardownHandler

	reconciler []reconciler
}

func NewMultiClusterObjectSetPhaseController(
	log logr.Logger, scheme *runtime.Scheme,
	dynamicCache dynamicCache,
	uncachedClient client.Reader,
	class string,
	client client.Client, // client to get and update ObjectSetPhases (management cluster).
	targetWriter client.Writer, // client to patch objects with (hosted cluster).
	targetRESTMapper meta.RESTMapper,
	discoveryClient discovery.DiscoveryInterface,
) *GenericObjectSetPhaseController {
	return NewGenericObjectSetPhaseController(
		newGenericObjectSetPhase,
		newGenericObjectSet,
		ownerhandling.NewAnnotation(scheme, "package-operator.run/owners"),
		log, scheme, dynamicCache, uncachedClient,
		class, client, targetWriter,
		discoveryClient,
		// Do not prevent creation of objects in different namespaces on the target.
		validation.NewClusterPhaseValidator(targetRESTMapper, targetWriter),
	)
}

func NewMultiClusterClusterObjectSetPhaseController(
	log logr.Logger, scheme *runtime.Scheme,
	dynamicCache dynamicCache,
	uncachedClient client.Reader,
	class string,
	client client.Client, // client to get and update ObjectSetPhases (management cluster).
	targetWriter client.Writer, // client to patch objects with (hosted cluster).
	targetRESTMapper meta.RESTMapper,
	discoveryClient discovery.DiscoveryInterface,
) *GenericObjectSetPhaseController {
	return NewGenericObjectSetPhaseController(
		newGenericClusterObjectSetPhase,
		newGenericClusterObjectSet,
		ownerhandling.NewAnnotation(scheme, "package-operator.run/owners"),
		log, scheme, dynamicCache, uncachedClient,
		class, client, targetWriter,
		discoveryClient,
		validation.NewClusterPhaseValidator(targetRESTMapper, targetWriter),
	)
}

func NewSameClusterObjectSetPhaseController(
	log logr.Logger, scheme *runtime.Scheme,
	dynamicCache dynamicCache,
	uncachedClient client.Reader,
	class string,
	client client.Client, // client to get and update ObjectSetPhases.
	restMapper meta.RESTMapper,
	discoveryClient discovery.DiscoveryInterface,
) *GenericObjectSetPhaseController {
	return NewGenericObjectSetPhaseController(
		newGenericObjectSetPhase,
		newGenericObjectSet,
		ownerhandling.NewNative(scheme),
		log, scheme, dynamicCache, uncachedClient,
		class, client, client,
		discoveryClient,
		validation.NewNamespacedPhaseValidator(restMapper, client),
	)
}

func NewSameClusterClusterObjectSetPhaseController(
	log logr.Logger, scheme *runtime.Scheme,
	dynamicCache dynamicCache,
	uncachedClient client.Reader,
	class string,
	client client.Client, // client to get and update ObjectSetPhases.
	restMapper meta.RESTMapper,
	discoveryClient discovery.DiscoveryInterface,
) *GenericObjectSetPhaseController {
	return NewGenericObjectSetPhaseController(
		newGenericClusterObjectSetPhase,
		newGenericClusterObjectSet,
		ownerhandling.NewNative(scheme),
		log, scheme, dynamicCache, uncachedClient,
		class, client, client,
		discoveryClient,
		validation.NewClusterPhaseValidator(restMapper, client),
	)
}

func NewGenericObjectSetPhaseController(
	newObjectSetPhase genericObjectSetPhaseFactory,
	newObjectSet genericObjectSetFactory,
	ownerStrategy ownerStrategy,
	log logr.Logger, scheme *runtime.Scheme,
	dynamicCache dynamicCache,
	uncachedClient client.Reader,
	class string,
	client client.Client, // client to get and update ObjectSetPhases.
	targetWriter client.Writer, // client to patch objects with.
	discoveryClient discovery.DiscoveryInterface,
	phaseValidator phaseValidator,
) *GenericObjectSetPhaseController {
	controller := &GenericObjectSetPhaseController{
		newObjectSetPhase: newObjectSetPhase,

		class:  class,
		log:    log,
		scheme: scheme,

		client:        client,
		dynamicCache:  dynamicCache,
		ownerStrategy: ownerStrategy,
	}

	phaseReconciler := newObjectSetPhaseReconciler(
		scheme,
		controllers.NewPhaseReconciler(
			scheme, targetWriter, dynamicCache, uncachedClient, ownerStrategy,
			machinery.NewPhaseEngine(
				machinery.NewObjectEngine(dynamicCache, uncachedClient, targetWriter, ownerStrategy,
					machinery.NewComparator(ownerStrategy, discoveryClient, "package-operator"),
					"package-operator", "package-operator.run",
				),
				phaseValidator,
			),
		),
		controllers.NewPreviousRevisionLookup(
			scheme, func(s *runtime.Scheme) controllers.PreviousObjectSet {
				return newObjectSet(s)
			}, client).Lookup,
		ownerStrategy,
	)
	controller.teardownHandler = phaseReconciler
	controller.reconciler = []reconciler{
		phaseReconciler,
	}

	return controller
}

func (c *GenericObjectSetPhaseController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("ObjectSetPhase", req.String())
	defer log.Info("reconciled")
	ctx = logr.NewContext(ctx, log)

	objectSetPhase := c.newObjectSetPhase(c.scheme)
	if err := c.client.Get(
		ctx, req.NamespacedName, objectSetPhase.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if objectSetPhase.GetClass() != c.class {
		return ctrl.Result{}, nil
	}

	if !objectSetPhase.ClientObject().GetDeletionTimestamp().IsZero() {
		if err := c.handleDeletionAndArchival(ctx, objectSetPhase); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, c.updateStatus(ctx, objectSetPhase)
	}

	if err := controllers.EnsureCachedFinalizer(ctx, c.client, objectSetPhase.ClientObject()); err != nil {
		return ctrl.Result{}, err
	}

	var (
		res ctrl.Result
		err error
	)
	for _, r := range c.reconciler {
		res, err = r.Reconcile(ctx, objectSetPhase)
		if err != nil || !res.IsZero() {
			break
		}
	}

	if err != nil {
		return controllers.UpdateObjectSetOrPhaseStatusFromError(ctx, objectSetPhase, err,
			func(ctx context.Context) error {
				return c.updateStatus(ctx, objectSetPhase)
			})
	}

	c.reportPausedCondition(ctx, objectSetPhase)
	return res, c.updateStatus(ctx, objectSetPhase)
}

func (c *GenericObjectSetPhaseController) reportPausedCondition(
	_ context.Context, objectSetPhase genericObjectSetPhase,
) {
	if objectSetPhase.IsPaused() {
		meta.SetStatusCondition(objectSetPhase.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetPhasePaused,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: objectSetPhase.GetGeneration(),
			Reason:             "Paused",
			Message:            "Lifecycle state set to paused.",
		})
	} else {
		meta.RemoveStatusCondition(objectSetPhase.GetConditions(), corev1alpha1.ObjectSetPaused)
	}
}

func (c *GenericObjectSetPhaseController) updateStatus(
	ctx context.Context, objectSetPhase genericObjectSetPhase,
) error {
	if err := c.client.Status().Update(ctx, objectSetPhase.ClientObject()); err != nil {
		return fmt.Errorf("updating ObjectSetPhase status: %w", err)
	}
	return nil
}

func (c *GenericObjectSetPhaseController) handleDeletionAndArchival(
	ctx context.Context, objectSetPhase genericObjectSetPhase,
) error {
	done := true

	// When removing the finalizer this function may be called one last time.
	// .Teardown may allocate new watches and leave dangling watches.
	if controllerutil.ContainsFinalizer(
		objectSetPhase.ClientObject(), constants.CachedFinalizer) {
		var err error
		done, err = c.teardownHandler.Teardown(ctx, objectSetPhase)
		if err != nil {
			return fmt.Errorf("error tearing down during deletion: %w", err)
		}
	}

	if !done {
		// don't remove finalizer before deletion is done
		return nil
	}

	return controllers.FreeCacheAndRemoveFinalizer(ctx, c.client, objectSetPhase.ClientObject(), c.dynamicCache)
}

func (c *GenericObjectSetPhaseController) SetupWithManager(
	mgr ctrl.Manager,
) error {
	objectSetPhase := c.newObjectSetPhase(c.scheme).ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(objectSetPhase).
		WatchesRawSource(
			c.dynamicCache.Source(
				c.ownerStrategy.EnqueueRequestForOwner(objectSetPhase, mgr.GetRESTMapper(), false),
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
		).Complete(c)
}
