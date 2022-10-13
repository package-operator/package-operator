package objectsetphases

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/ownerhandling"
)

type reconciler interface {
	Reconcile(ctx context.Context, objectSet genericObjectSetPhase) (ctrl.Result, error)
}

type dynamicCache interface {
	client.Reader
	Source() source.Source
	Free(ctx context.Context, obj client.Object) error
	Watch(ctx context.Context, owner client.Object, obj runtime.Object) error
}

type ownerStrategy interface {
	IsController(owner, obj metav1.Object) bool
	ReleaseController(obj metav1.Object)
	RemoveOwner(owner, obj metav1.Object)
	SetControllerReference(owner, obj metav1.Object) error
	EnqueueRequestForOwner(
		ownerType client.Object, isController bool,
	) handler.EventHandler
}

type teardownHandler interface {
	Teardown(
		ctx context.Context, objectSetPhase genericObjectSetPhase,
	) (cleanupDone bool, err error)
}

// Generic reconciler for both ObjectSetPhase and ClusterObjectSetPhase objects.
type GenericObjectSetPhaseController struct {
	newObjectSetPhase genericObjectSetPhaseFactory

	class  string // Phase class this controller is operating for.
	log    logr.Logger
	scheme *runtime.Scheme
	// ownerStrategy ownerStrategy
	client          client.Client // client to get and update ObjectSetPhases.
	dynamicCache    dynamicCache
	ownerStrategy   ownerStrategy
	teardownHandler teardownHandler

	reconciler []reconciler
}

func NewMultiClusterObjectSetPhaseController(
	log logr.Logger, scheme *runtime.Scheme,
	dynamicCache dynamicCache,
	class string,
	client client.Client, // client to get and update ObjectSetPhases.
	targetWriter client.Writer, // client to patch objects with.
) *GenericObjectSetPhaseController {
	return NewGenericObjectSetPhaseController(
		newGenericObjectSetPhase,
		newGenericObjectSet,
		ownerhandling.NewAnnotation(scheme),
		log, scheme, dynamicCache,
		class, client, targetWriter,
	)
}

func NewMultiClusterClusterObjectSetPhaseController(
	log logr.Logger, scheme *runtime.Scheme,
	dynamicCache dynamicCache,
	class string,
	client client.Client, // client to get and update ObjectSetPhases.
	targetWriter client.Writer, // client to patch objects with.
) *GenericObjectSetPhaseController {
	return NewGenericObjectSetPhaseController(
		newGenericClusterObjectSetPhase,
		newGenericClusterObjectSet,
		ownerhandling.NewAnnotation(scheme),
		log, scheme, dynamicCache,
		class, client, targetWriter,
	)
}

func NewSameClusterObjectSetPhaseController(
	log logr.Logger, scheme *runtime.Scheme,
	dynamicCache dynamicCache,
	class string,
	client client.Client, // client to get and update ObjectSetPhases.
) *GenericObjectSetPhaseController {
	return NewGenericObjectSetPhaseController(
		newGenericObjectSetPhase,
		newGenericObjectSet,
		ownerhandling.NewNative(scheme),
		log, scheme, dynamicCache,
		class, client, client,
	)
}

func NewSameClusterClusterObjectSetPhaseController(
	log logr.Logger, scheme *runtime.Scheme,
	dynamicCache dynamicCache,
	class string,
	client client.Client, // client to get and update ObjectSetPhases.
) *GenericObjectSetPhaseController {
	return NewGenericObjectSetPhaseController(
		newGenericClusterObjectSetPhase,
		newGenericClusterObjectSet,
		ownerhandling.NewNative(scheme),
		log, scheme, dynamicCache,
		class, client, client,
	)
}

func NewGenericObjectSetPhaseController(
	newObjectSetPhase genericObjectSetPhaseFactory,
	newObjectSet genericObjectSetFactory,
	ownerStrategy ownerStrategy,
	log logr.Logger, scheme *runtime.Scheme,
	dynamicCache dynamicCache,
	class string,
	client client.Client, // client to get and update ObjectSetPhases.
	targetWriter client.Writer, // client to patch objects with.
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
			scheme, targetWriter, dynamicCache, ownerStrategy),
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

	defer log.Info("reconciled")

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
		return res, err
	}

	c.reportPausedCondition(ctx, objectSetPhase)
	return res, c.updateStatus(ctx, objectSetPhase)
}

func (c *GenericObjectSetPhaseController) reportPausedCondition(_ context.Context, objectSetPhase genericObjectSetPhase) {
	if objectSetPhase.IsPaused() {
		meta.SetStatusCondition(objectSetPhase.GetConditions(), metav1.Condition{
			Type:    corev1alpha1.ObjectSetPhasePaused,
			Status:  metav1.ConditionTrue,
			Reason:  "Paused",
			Message: "Lifecycle state set to paused.",
		})
	} else {
		meta.RemoveStatusCondition(objectSetPhase.GetConditions(), corev1alpha1.ObjectSetPaused)
	}
}

func (c *GenericObjectSetPhaseController) updateStatus(
	ctx context.Context, objectSetPhase genericObjectSetPhase) error {
	if err := c.client.Status().Update(ctx, objectSetPhase.ClientObject()); err != nil {
		return fmt.Errorf("updating ObjectSetPhase status: %w", err)
	}
	return nil
}

func (c *GenericObjectSetPhaseController) handleDeletionAndArchival(
	ctx context.Context, objectSetPhase genericObjectSetPhase,
) error {
	done, err := c.teardownHandler.Teardown(ctx, objectSetPhase)
	if err != nil {
		return fmt.Errorf("error tearing down during deletion: %w", err)
	}

	if !done {
		// don't remove finalizer before deletion is done
		return nil
	}

	if err := controllers.FreeCacheAndRemoveFinalizer(
		ctx, c.client, objectSetPhase.ClientObject(), c.dynamicCache); err != nil {
		return err
	}
	return nil
}

func (c *GenericObjectSetPhaseController) SetupWithManager(
	mgr ctrl.Manager) error {

	objectSetPhase := c.newObjectSetPhase(c.scheme).ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(objectSetPhase).
		Watches(c.dynamicCache.Source(), c.ownerStrategy.EnqueueRequestForOwner(objectSetPhase, false)).
		Complete(c)
}
