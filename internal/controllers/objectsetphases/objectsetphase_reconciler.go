package objectsetphases

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/util/flowcontrol"
	"pkg.package-operator.run/boxcutter"
	"pkg.package-operator.run/boxcutter/machinery"
	"pkg.package-operator.run/boxcutter/machinery/types"
	"pkg.package-operator.run/boxcutter/managedcache"
	"pkg.package-operator.run/boxcutter/ownerhandling"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/controllers"
	internalprobing "package-operator.run/internal/probing"

	// TODO why does goimport INSIST on having an alias for this import?
	// if i remove this then it is aliased as boxcutter
	boxcutterutil "package-operator.run/internal/controllers/boxcutterutil"
)

// objectSetPhaseReconciler reconciles objects within a phase.
type objectSetPhaseReconciler struct {
	scheme          *runtime.Scheme
	discoveryClient discovery.DiscoveryInterface
	restMapper      meta.RESTMapper
	phaseValidator  machinery.PhaseValidator

	accessManager           managedcache.ObjectBoundAccessManager[client.Object]
	phaseReconcilerFactory  controllers.PhaseReconcilerFactory
	lookupPreviousRevisions lookupPreviousRevisions
	ownerStrategy           ownerhandling.OwnerStrategy
	backoff                 *flowcontrol.Backoff
}

func newObjectSetPhaseReconciler(
	scheme *runtime.Scheme,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	phaseReconcilerFactory controllers.PhaseReconcilerFactory,
	lookupPreviousRevisions lookupPreviousRevisions,
	ownerStrategy ownerhandling.OwnerStrategy,
) *objectSetPhaseReconciler {
	var cfg objectSetPhaseReconcilerConfig

	cfg.Default()

	return &objectSetPhaseReconciler{
		scheme:                  scheme,
		accessManager:           accessManager,
		phaseReconcilerFactory:  phaseReconcilerFactory,
		lookupPreviousRevisions: lookupPreviousRevisions,
		ownerStrategy:           ownerStrategy,
		backoff:                 cfg.GetBackoff(),
	}
}

type lookupPreviousRevisions func(
	ctx context.Context, owner controllers.PreviousOwner,
) ([]controllers.PreviousObjectSet, error)

func (r *objectSetPhaseReconciler) Reconcile(
	ctx context.Context, objectSetPhase adapters.ObjectSetPhaseAccessor,
) (res ctrl.Result, err error) {
	defer r.backoff.GC()

	controllers.DeleteMappedConditions(ctx, objectSetPhase.GetConditions())

	previous, err := r.lookupPreviousRevisions(ctx, objectSetPhase)
	if err != nil {
		return res, fmt.Errorf("lookup previous revisions: %w", err)
	}

	probe, err := internalprobing.Parse(
		ctx, objectSetPhase.GetAvailabilityProbes())
	if err != nil {
		return res, fmt.Errorf("parsing probes: %w", err)
	}

	objectsInPhase := []client.Object{}
	for _, object := range objectSetPhase.GetPhase().Objects {
		objectsInPhase = append(objectsInPhase, &object.Object)
	}
	cache, err := r.accessManager.GetWithUser(
		ctx,
		constants.StaticCacheOwner(),
		objectSetPhase.ClientObject(),
		objectsInPhase,
	)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("preparing cache: %w", err)
	}

	phaseEngine, err := boxcutter.NewPhaseEngine(boxcutter.RevisionEngineOptions{
		Scheme:          r.scheme,
		FieldOwner:      constants.FieldOwner,
		SystemPrefix:    constants.SystemPrefix,
		DiscoveryClient: r.discoveryClient,
		RestMapper:      r.restMapper,
		Writer:          cache,
		Reader:          cache,
		OwnerStrategy:   r.ownerStrategy,
		PhaseValidator:  r.phaseValidator,
	})
	if err != nil {
		return res, fmt.Errorf("preparing PhaseEngine: %w", err)
	}

	apiPhase := objectSetPhase.GetPhase()
	phaseObjects := make([]unstructured.Unstructured, len(apiPhase.Objects))
	phaseReconcileOptions := make([]types.PhaseReconcileOption, len(apiPhase.Objects))
	for i := range apiPhase.Objects {
		phaseObjects[i] = apiPhase.Objects[i].Object
		phaseReconcileOptions[i] = types.WithObjectReconcileOptions(
			&apiPhase.Objects[i].Object,
			boxcutterutil.TranslateCollisionProtection(apiPhase.Objects[i].CollisionProtection),
			// howto probing?
			// howto conditionmapping?
			// apiPhase.Objects[i].ConditionMappings
			// howto pausing?
		)
	}

	result, err := phaseEngine.Reconcile(ctx,
		objectSetPhase.ClientObject(),
		objectSetPhase.GetRevision(),
		types.Phase{
			Name:    apiPhase.Name,
			Objects: phaseObjects,
		},
	)
	if controllers.IsExternalResourceNotFound(err) {
		id := string(objectSetPhase.ClientObject().GetUID())

		r.backoff.Next(id, r.backoff.Clock.Now())

		return ctrl.Result{
			RequeueAfter: r.backoff.Get(id),
		}, nil
	} else if err != nil {
		return res, fmt.Errorf("PhaseEngine reconcile error: %w", err)
	}

	// actualObjects, probingResult, err := phaseReconciler.ReconcilePhase(
	// 	ctx, objectSetPhase, objectSetPhase.GetPhase(), probe, previous)
	// if controllers.IsExternalResourceNotFound(err) {
	// 	id := string(objectSetPhase.ClientObject().GetUID())

	// 	r.backoff.Next(id, r.backoff.Clock.Now())

	// 	return ctrl.Result{
	// 		RequeueAfter: r.backoff.Get(id),
	// 	}, nil
	// } else if err != nil {
	// 	return res, err
	// }

	if err := r.reportOwnActiveObjects(ctx, objectSetPhase, result.GetObjects()); err != nil {
		return res, fmt.Errorf("reporting active objects: %w", err)
	}
	for _, objr := range result.GetObjects() {
		// objr.Success()
		// -> need a method that just exposes if we control the object.
	}

	if !probingResult.IsZero() {
		meta.SetStatusCondition(
			objectSetPhase.GetConditions(), metav1.Condition{
				Type:               corev1alpha1.ObjectSetAvailable,
				Status:             metav1.ConditionFalse,
				Reason:             "ProbeFailure",
				Message:            probingResult.StringWithoutPhase(),
				ObservedGeneration: objectSetPhase.ClientObject().GetGeneration(),
			})

		return res, nil
	}

	meta.SetStatusCondition(objectSetPhase.GetConditions(), metav1.Condition{
		Type:               corev1alpha1.ObjectSetPhaseAvailable,
		Status:             metav1.ConditionTrue,
		Reason:             "Available",
		Message:            "Object is available and passes all probes.",
		ObservedGeneration: objectSetPhase.ClientObject().GetGeneration(),
	})

	return ctrl.Result{}, nil
}

func (r *objectSetPhaseReconciler) Teardown(
	ctx context.Context, objectSetPhase adapters.ObjectSetPhaseAccessor,
) (cleanupDone bool, err error) {
	// objectSetPhase is deleted with the `orphan` cascade option, so we don't need to delete the owned objects.
	if controllerutil.ContainsFinalizer(objectSetPhase.ClientObject(), "orphan") {
		return true, nil
	}

	objectsInPhase := []client.Object{}
	for _, object := range objectSetPhase.GetPhase().Objects {
		objectsInPhase = append(objectsInPhase, &object.Object)
	}
	cache, err := r.accessManager.GetWithUser(
		ctx,
		constants.StaticCacheOwner(),
		objectSetPhase.ClientObject(),
		objectsInPhase,
	)
	if err != nil {
		return false, fmt.Errorf("preparing cache: %w", err)
	}
	phaseReconciler := r.phaseReconcilerFactory.New(cache)

	cleanupDone, err = phaseReconciler.TeardownPhase(
		ctx, objectSetPhase, objectSetPhase.GetPhase())
	if err != nil {
		return false, err
	}
	if !cleanupDone {
		return false, nil
	}

	if err := r.accessManager.FreeWithUser(
		ctx,
		constants.StaticCacheOwner(),
		objectSetPhase.ClientObject(),
	); err != nil {
		return false, fmt.Errorf("freewithuser: %w", err)
	}

	return true, nil
}

// Sets .status.activeObjects to all objects actively reconciled and controlled by this Phase.
func (r *objectSetPhaseReconciler) reportOwnActiveObjects(
	ctx context.Context, objectSetPhase adapters.ObjectSetPhaseAccessor, actualObjects []machinery.ObjectResult,
) error {

	activeObjects, err := controllers.GetControllerOf(
		ctx, r.scheme, r.ownerStrategy,
		objectSetPhase.ClientObject(), actualObjects)
	if err != nil {
		return err
	}
	objectSetPhase.SetStatusControllerOf(activeObjects)
	return nil
}

type objectSetPhaseReconcilerConfig struct {
	controllers.BackoffConfig
}

func (c *objectSetPhaseReconcilerConfig) Option(opts ...objectSetPhaseReconcilerOption) {
	for _, opt := range opts {
		opt.ConfigureObjectSetPhaseReconciler(c)
	}
}

func (c *objectSetPhaseReconcilerConfig) Default() {
	c.BackoffConfig.Default()
}

type objectSetPhaseReconcilerOption interface {
	ConfigureObjectSetPhaseReconciler(*objectSetPhaseReconcilerConfig)
}
