package objectsetphases

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/controllers/boxcutterutil"
	"package-operator.run/internal/preflight"

	"pkg.package-operator.run/boxcutter/managedcache"
	"pkg.package-operator.run/boxcutter/ownerhandling"
	"pkg.package-operator.run/boxcutter/validation"
)

type reconciler interface {
	Reconcile(ctx context.Context, objectSet adapters.ObjectSetPhaseAccessor) (ctrl.Result, error)
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
}

type teardownHandler interface {
	Teardown(
		ctx context.Context, objectSetPhase adapters.ObjectSetPhaseAccessor,
	) (cleanupDone bool, err error)
}

type preflightChecker interface {
	Check(
		ctx context.Context, owner, obj client.Object,
	) (violations []preflight.Violation, err error)
}

// Generic reconciler for both ObjectSetPhase and ClusterObjectSetPhase objects.
type GenericObjectSetPhaseController struct {
	newObjectSetPhase adapters.ObjectSetPhaseFactory

	class           string // Phase class this controller is operating for.
	log             logr.Logger
	scheme          *runtime.Scheme
	client          client.Client // client to get and update ObjectSetPhases.
	accessManager   managedcache.ObjectBoundAccessManager[client.Object]
	ownerStrategy   ownerStrategy
	teardownHandler teardownHandler

	reconciler []reconciler
}

func NewMultiClusterObjectSetPhaseController(
	log logr.Logger, scheme *runtime.Scheme,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	uncachedClient client.Reader,
	class string,
	client client.Client, // client to get and update ObjectSetPhases (management cluster).
	targetWriter client.Writer, // client to patch objects with (hosted cluster).
	targetRESTMapper meta.RESTMapper,
	discoveryClient discovery.DiscoveryInterface,
) *GenericObjectSetPhaseController {
	return NewGenericObjectSetPhaseController(
		adapters.NewObjectSetPhaseAccessor,
		adapters.NewObjectSet,
		ownerhandling.NewAnnotation(scheme, constants.OwnerStrategyAnnotationKey),
		log, scheme, accessManager, uncachedClient,
		class, client,
		preflight.NewAPIExistence(
			targetRESTMapper,
			preflight.List{
				preflight.NewNoOwnerReferences(targetRESTMapper),
				preflight.NewDryRun(targetWriter),
			},
		),
		discoveryClient,
		targetRESTMapper,
		validation.NewClusterPhaseValidator(targetRESTMapper, targetWriter),
	)
}

func NewMultiClusterClusterObjectSetPhaseController(
	log logr.Logger, scheme *runtime.Scheme,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	uncachedClient client.Reader,
	class string,
	client client.Client, // client to get and update ObjectSetPhases (management cluster).
	targetWriter client.Writer, // client to patch objects with (hosted cluster).
	targetRESTMapper meta.RESTMapper,
	discoveryClient discovery.DiscoveryInterface,
) *GenericObjectSetPhaseController {
	return NewGenericObjectSetPhaseController(
		adapters.NewClusterObjectSetPhaseAccessor,
		adapters.NewClusterObjectSet,
		ownerhandling.NewAnnotation(scheme, constants.OwnerStrategyAnnotationKey),
		log, scheme, accessManager, uncachedClient,
		class, client,
		preflight.NewAPIExistence(
			targetRESTMapper,
			preflight.List{
				preflight.NewDryRun(targetWriter),
				preflight.NewNoOwnerReferences(targetRESTMapper),
			},
		),
		discoveryClient,
		targetRESTMapper,
		validation.NewClusterPhaseValidator(targetRESTMapper, targetWriter),
	)
}

func NewSameClusterObjectSetPhaseController(
	log logr.Logger, scheme *runtime.Scheme,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	uncachedClient client.Reader,
	class string,
	client client.Client, // client to get and update ObjectSetPhases.
	restMapper meta.RESTMapper,
	discoveryClient discovery.DiscoveryInterface,
) *GenericObjectSetPhaseController {
	return NewGenericObjectSetPhaseController(
		adapters.NewObjectSetPhaseAccessor,
		adapters.NewObjectSet,
		ownerhandling.NewNative(scheme),
		log, scheme, accessManager, uncachedClient,
		class, client,
		preflight.NewAPIExistence(
			restMapper,
			preflight.List{
				preflight.NewNamespaceEscalation(restMapper),
				preflight.NewDryRun(client),
				preflight.NewNoOwnerReferences(restMapper),
			},
		),
		discoveryClient,
		restMapper,
		validation.NewNamespacedPhaseValidator(restMapper, client),
	)
}

func NewSameClusterClusterObjectSetPhaseController(
	log logr.Logger, scheme *runtime.Scheme,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	uncachedClient client.Reader,
	class string,
	client client.Client, // client to get and update ObjectSetPhases.
	restMapper meta.RESTMapper,
	discoveryClient discovery.DiscoveryInterface,
) *GenericObjectSetPhaseController {
	return NewGenericObjectSetPhaseController(
		adapters.NewClusterObjectSetPhaseAccessor,
		adapters.NewClusterObjectSet,
		ownerhandling.NewNative(scheme),
		log, scheme, accessManager, uncachedClient,
		class, client,
		preflight.NewAPIExistence(
			restMapper,
			preflight.List{
				preflight.NewDryRun(client),
				preflight.NewNoOwnerReferences(restMapper),
			},
		),
		discoveryClient,
		restMapper,
		validation.NewNamespacedPhaseValidator(restMapper, client),
	)
}

func NewGenericObjectSetPhaseController(
	newObjectSetPhase adapters.ObjectSetPhaseFactory,
	newObjectSet adapters.ObjectSetAccessorFactory,
	ownerStrategy ownerhandling.OwnerStrategy,
	log logr.Logger, scheme *runtime.Scheme,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	uncachedClient client.Reader,
	class string,
	client client.Client, // client to get and update ObjectSetPhases.
	preflightChecker preflightChecker,
	discoveryClient discovery.DiscoveryInterface,
	targetRESTMapper meta.RESTMapper,
	phaseValidator *validation.PhaseValidator,
) *GenericObjectSetPhaseController {
	controller := &GenericObjectSetPhaseController{
		newObjectSetPhase: newObjectSetPhase,

		class:  class,
		log:    log,
		scheme: scheme,

		client:        client,
		ownerStrategy: ownerStrategy,
		accessManager: accessManager,
	}
	phaseReconciler := newObjectSetPhaseReconciler(
		scheme,
		accessManager,
		boxcutterutil.NewPhaseEngineFactory(
			scheme, discoveryClient, targetRESTMapper, ownerStrategy, phaseValidator),
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
	_ context.Context, objectSetPhase adapters.ObjectSetPhaseAccessor,
) {
	if objectSetPhase.IsSpecPaused() {
		meta.SetStatusCondition(objectSetPhase.GetStatusConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetPhasePaused,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: objectSetPhase.GetGeneration(),
			Reason:             "Paused",
			Message:            "Lifecycle state set to paused.",
		})
	} else {
		meta.RemoveStatusCondition(objectSetPhase.GetStatusConditions(), corev1alpha1.ObjectSetPaused)
	}
}

func (c *GenericObjectSetPhaseController) updateStatus(
	ctx context.Context, objectSetPhase adapters.ObjectSetPhaseAccessor,
) error {
	if err := c.client.Status().Update(ctx, objectSetPhase.ClientObject()); err != nil {
		return fmt.Errorf("updating ObjectSetPhase status: %w", err)
	}
	return nil
}

func (c *GenericObjectSetPhaseController) handleDeletionAndArchival(
	ctx context.Context, objectSetPhase adapters.ObjectSetPhaseAccessor,
) error {
	done := true

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

	return controllers.RemoveCacheFinalizer(ctx, c.client, objectSetPhase.ClientObject())
}

func (c *GenericObjectSetPhaseController) SetupWithManager(
	mgr ctrl.Manager,
) error {
	objectSetPhase := c.newObjectSetPhase(c.scheme).ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(objectSetPhase).
		WatchesRawSource(
			c.accessManager.Source(
				c.ownerStrategy.EnqueueRequestForOwner(objectSetPhase, mgr.GetRESTMapper(), false),
				predicate.NewPredicateFuncs(func(object client.Object) bool {
					c.log.V(constants.LogLevelDebug).Info(
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
