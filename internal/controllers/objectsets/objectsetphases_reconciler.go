package objectsets

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/flowcontrol"
	"pkg.package-operator.run/boxcutter"
	"pkg.package-operator.run/boxcutter/machinery"
	"pkg.package-operator.run/boxcutter/machinery/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	internalprobing "package-operator.run/internal/probing"

	"package-operator.run/internal/controllers/boxcutterutil"

	"pkg.package-operator.run/boxcutter/managedcache"
	"pkg.package-operator.run/boxcutter/ownerhandling"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/preflight"
)

// objectSetPhasesReconciler reconciles all phases within an ObjectSet.
type objectSetPhasesReconciler struct {
	cfg                     objectSetPhasesReconcilerConfig
	scheme                  *runtime.Scheme
	accessManager           managedcache.ObjectBoundAccessManager[client.Object]
	revisionEngineFactory   boxcutterutil.RevisionEngineFactory
	remotePhase             remotePhaseReconciler
	lookupPreviousRevisions lookupPreviousRevisions
	ownerStrategy           boxcutterutil.OwnerStrategy
	preflightChecker        phasesChecker
	backoff                 *flowcontrol.Backoff
}

type ownerStrategy interface {
	IsController(owner, obj metav1.Object) bool
	IsOwner(owner, obj metav1.Object) bool
}

type phasesChecker interface {
	Check(
		ctx context.Context, phases []corev1alpha1.ObjectSetTemplatePhase,
	) (violations []preflight.Violation, err error)
}

func newObjectSetPhasesReconciler(
	scheme *runtime.Scheme,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	revisionEngineFactory boxcutterutil.RevisionEngineFactory,
	remotePhase remotePhaseReconciler,
	lookupPreviousRevisions lookupPreviousRevisions,
	checker phasesChecker,
	opts ...objectSetPhasesReconcilerOption,
) *objectSetPhasesReconciler {
	var cfg objectSetPhasesReconcilerConfig

	cfg.Option(opts...)
	cfg.Default()

	return &objectSetPhasesReconciler{
		cfg:                     cfg,
		scheme:                  scheme,
		accessManager:           accessManager,
		revisionEngineFactory:   revisionEngineFactory,
		remotePhase:             remotePhase,
		lookupPreviousRevisions: lookupPreviousRevisions,
		ownerStrategy:           ownerhandling.NewNative(scheme),
		preflightChecker:        checker,
		backoff:                 cfg.GetBackoff(),
	}
}

type remotePhaseReconciler interface {
	Reconcile(
		ctx context.Context, objectSet adapters.ObjectSetAccessor,
		phase corev1alpha1.ObjectSetTemplatePhase,
	) (machinery.PhaseResult, error)
	Teardown(
		ctx context.Context, objectSet adapters.ObjectSetAccessor,
		phase corev1alpha1.ObjectSetTemplatePhase,
	) (cleanupDone bool, err error)
}

type lookupPreviousRevisions func(
	ctx context.Context, owner controllers.PreviousOwner,
) ([]controllers.PreviousObjectSet, error)

func (r *objectSetPhasesReconciler) Reconcile(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
) (res ctrl.Result, err error) {
	defer r.backoff.GC()
	log := logr.FromContextOrDiscard(ctx).WithName("objectSetPhasesReconciler")
	log.Info("reconcile")
	defer log.Info("reconciled")

	violations, err := r.preflightChecker.Check(ctx, objectSet.GetSpecPhases())
	if err != nil {
		return res, err
	}
	if len(violations) > 0 {
		preflightErr := &preflight.Error{
			Violations: violations,
		}
		return res, preflightErr
	}

	controllers.DeleteMappedConditions(ctx, objectSet.GetStatusConditions())

	reconcileResult, err := r.reconcile(ctx, objectSet)

	if controllers.IsExternalResourceNotFound(err) {
		id := string(objectSet.ClientObject().GetUID())

		r.backoff.Next(id, r.backoff.Clock.Now())

		return ctrl.Result{
			RequeueAfter: r.backoff.Get(id),
		}, nil
	} else if err != nil {
		return res, err
	}

	return res, r.updateStatus(objectSet, reconcileResult)
}

func (r *objectSetPhasesReconciler) reconcile(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
) (machinery.RevisionResult, error) {
	log := logr.FromContextOrDiscard(ctx).WithName("objectSetPhasesReconciler")

	log.Info("getting cache accessor")
	cache, err := r.accessManager.GetWithUser(
		ctx,
		constants.StaticCacheOwner(),
		objectSet.ClientObject(),
		aggregateLocalObjects(objectSet),
	)
	if err != nil {
		return nil, fmt.Errorf("getting cache: %w", err)
	}

	revisionEngine, err := r.revisionEngineFactory.New(cache)
	if err != nil {
		return nil, fmt.Errorf("constructing revision engine: %w", err)
	}

	revision, err := r.buildRevision(ctx, objectSet)
	if err != nil {
		return nil, fmt.Errorf("building revision: %w", err)
	}

	reconcileResult, err := revisionEngine.Reconcile(ctx, revision)
	if err != nil {
		return nil, err
	}
	return reconcileResult, nil
}

func (r *objectSetPhasesReconciler) updateStatus(
	objectSet adapters.ObjectSetAccessor,
	reconcileResult machinery.RevisionResult,
) error {
	var actualObjects []machinery.Object
	var controllerOf []corev1alpha1.ControlledObjectReference
	for _, phaseRes := range reconcileResult.GetPhases() {
		for _, obj := range phaseRes.GetObjects() {
			actualObjects = append(actualObjects, obj.Object())
		}

		if remoteRes, ok := phaseRes.(*remotePhaseResult); ok {
			controllerOf = append(controllerOf, remoteRes.GetControllerOf()...)
		} else {
			controllerOf = append(controllerOf,
				boxcutterutil.GetControllerOf(r.ownerStrategy, objectSet.ClientObject(), phaseRes)...)
		}
	}

	if err := mapConditions(actualObjects, objectSet); err != nil {
		return err
	}
	objectSet.SetStatusControllerOf(controllerOf)

	inTransition := isObjectSetInTransition(objectSet, controllerOf)
	if inTransition {
		meta.SetStatusCondition(objectSet.GetStatusConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetInTransition,
			Status:             metav1.ConditionTrue,
			Reason:             "InTransition",
			Message:            "ObjectSet is still rolling out or is being replaced by a newer version.",
			ObservedGeneration: objectSet.ClientObject().GetGeneration(),
		})
	} else {
		meta.RemoveStatusCondition(objectSet.GetStatusConditions(), corev1alpha1.ObjectSetInTransition)
	}

	if !reconcileResult.IsComplete() {
		meta.SetStatusCondition(objectSet.GetStatusConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetAvailable,
			Status:             metav1.ConditionFalse,
			Reason:             "ProbeFailure",
			Message:            reconcileResult.String(),
			ObservedGeneration: objectSet.ClientObject().GetGeneration(),
		})

		return nil
	}

	meta.SetStatusCondition(objectSet.GetStatusConditions(), metav1.Condition{
		Type:               corev1alpha1.ObjectSetAvailable,
		Status:             metav1.ConditionTrue,
		Reason:             "Available",
		Message:            "Object is available and passes all probes.",
		ObservedGeneration: objectSet.ClientObject().GetGeneration(),
	})

	if r.hasSurvivedDelay(objectSet) && !meta.IsStatusConditionTrue(
		*objectSet.GetStatusConditions(), corev1alpha1.ObjectSetSucceeded) &&
		// we don't want to record Succeeded during transition,
		// because the object may become Available due to external
		// (e.g. other ObjectSets) involvement.
		!inTransition {
		// Remember that this rollout worked!
		meta.SetStatusCondition(objectSet.GetStatusConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetSucceeded,
			Status:             metav1.ConditionTrue,
			Reason:             "RolloutSuccess",
			Message:            "ObjectSet rolled out all objects successfully and was Available at least once.",
			ObservedGeneration: objectSet.ClientObject().GetGeneration(),
		})
	}

	return nil
}

func (r *objectSetPhasesReconciler) buildRevision(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
) (boxcutter.Revision, error) {
	previous, err := r.lookupPreviousRevisions(ctx, objectSet)
	if err != nil {
		return nil, fmt.Errorf("lookup previous revisions: %w", err)
	}
	previousObjects := previousToObjects(previous)

	probe, err := internalprobing.Parse(ctx, objectSet.GetAvailabilityProbes())
	if err != nil {
		return nil, fmt.Errorf("parsing probes: %w", err)
	}

	revision := &adapters.RevisionAdapter{ObjectSet: objectSet}
	oo := types.WithOwner(objectSet.ClientObject(), r.ownerStrategy)
	revision.WithReconcileOptions(oo)

	for _, phase := range objectSet.GetSpecPhases() {
		var phaseOpts []types.PhaseReconcileOption

		for _, phaseObj := range phase.Objects {
			// TODO: remove namespace defaulting from PKO
			// Default namespace to the owners namespace
			if len(phaseObj.Object.GetNamespace()) == 0 {
				phaseObj.Object.SetNamespace(objectSet.ClientObject().GetNamespace())
			}

			labels := phaseObj.Object.GetLabels()
			if labels == nil {
				labels = map[string]string{}
			}
			// No need to apply dynamic cache label to ObjectSetPhases
			if len(phase.Class) == 0 {
				labels[constants.DynamicCacheLabel] = "True"
				phaseObj.Object.SetLabels(labels)
			}

			collisionProtection := boxcutterutil.TranslateCollisionProtection(phaseObj.CollisionProtection)

			// Take over existing PKO objects when bootstrapping (e.g., CRDs)
			// TODO: refactor the hardcoded PKO package name (there's another hardcoded reference in the bootstrap/init job)
			if len(os.Getenv(constants.ForceAdoptionEnvironmentVariable)) > 0 ||
				labels[manifestsv1alpha1.PackageLabel] == "package-operator" {
				collisionProtection = boxcutterutil.TranslateCollisionProtection(corev1alpha1.CollisionProtectionNone)
			}

			phaseOpts = append(phaseOpts, types.WithObjectReconcileOptions(
				&phaseObj.Object,
				collisionProtection,
				types.WithProbe(types.ProgressProbeType, probe),
				boxcutter.WithPreviousOwners(previousObjects),
			))
		}

		revision.WithReconcileOptions(types.WithPhaseReconcileOptions(phase.Name, phaseOpts...))
	}
	if objectSet.IsSpecPaused() {
		revision.WithReconcileOptions(types.WithPaused{})
	}

	return revision, nil
}

func previousToObjects(prev []controllers.PreviousObjectSet) []client.Object {
	objects := make([]client.Object, 0, len(prev))
	for _, p := range prev {
		objects = append(objects, p.ClientObject())
	}
	return objects
}

func (r *objectSetPhasesReconciler) Teardown(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
) (cleanupDone bool, err error) {
	// objectSet is deleted with the `orphan` cascade option, so we don't delete the owned objects
	if controllerutil.ContainsFinalizer(objectSet.ClientObject(), "orphan") {
		return true, nil
	}

	cache, err := r.accessManager.GetWithUser(
		ctx,
		constants.StaticCacheOwner(),
		objectSet.ClientObject(),
		aggregateLocalObjects(objectSet),
	)
	if err != nil {
		return false, fmt.Errorf("getting cache: %w", err)
	}

	revisionEngine, err := r.revisionEngineFactory.New(cache)
	if err != nil {
		return false, fmt.Errorf("constructing revision engine: %w", err)
	}
	revision := &adapters.RevisionAdapter{ObjectSet: objectSet}
	oo := types.WithOwner(objectSet.ClientObject(), r.ownerStrategy)
	revision.WithTeardownOptions(oo)

	teardownResult, err := revisionEngine.Teardown(ctx, revision)
	if err != nil {
		return false, err
	}
	if !teardownResult.IsComplete() {
		return false, nil
	}

	if err := r.accessManager.FreeWithUser(
		ctx,
		constants.StaticCacheOwner(),
		objectSet.ClientObject(),
	); err != nil {
		return false, fmt.Errorf("freewithuser: %w", err)
	}

	return true, nil
}

// Checks if an ObjectSet is in transition.
// An ObjectSet is in transition if it is not yet or no longer
// controlling all objects from spec.
// This state is true until the ObjectSet has finished a successful rollout
// or from the moment a newer revision is taking ownership until it has been archived.
func isObjectSetInTransition(
	objectSet adapters.ObjectSetAccessor,
	controllerOf []corev1alpha1.ControlledObjectReference,
) bool {
	if objectSet.IsSpecArchived() {
		return false
	}

	// Build a lookup map of all objects that may be managed by this ObjectSet.
	allObjectsThatMayBeUnderManagement := map[corev1alpha1.ControlledObjectReference]struct{}{}
	for _, phase := range objectSet.GetSpecPhases() {
		for _, obj := range phase.Objects {
			gvk := obj.Object.GroupVersionKind()
			ns := obj.Object.GetNamespace()
			if len(ns) == 0 {
				ns = objectSet.ClientObject().GetNamespace()
			}
			ref := corev1alpha1.ControlledObjectReference{
				Kind:      gvk.Kind,
				Group:     gvk.Group,
				Name:      obj.Object.GetName(),
				Namespace: ns,
			}
			allObjectsThatMayBeUnderManagement[ref] = struct{}{}
		}
	}

	for _, controlled := range controllerOf {
		if _, found := allObjectsThatMayBeUnderManagement[controlled]; found {
			// direct match
			delete(allObjectsThatMayBeUnderManagement, controlled)
			continue
		}

		// If object references originate from ObjectSetPhase API,
		// we might have cluster scoped objects with empty namespace.
		if len(controlled.Namespace) == 0 {
			// scan for object without namespace
			for objMayBeUnderManagement := range allObjectsThatMayBeUnderManagement {
				if objMayBeUnderManagement.Kind == controlled.Kind &&
					objMayBeUnderManagement.Group == controlled.Group &&
					objMayBeUnderManagement.Name == controlled.Name {
					controlled.Namespace = objMayBeUnderManagement.Namespace
					delete(allObjectsThatMayBeUnderManagement, controlled)
					break
				}
			}
		}
	}
	return len(allObjectsThatMayBeUnderManagement) > 0
}

func (r *objectSetPhasesReconciler) hasSurvivedDelay(objectSet adapters.ObjectSetAccessor) bool {
	availCond := meta.FindStatusCondition(*objectSet.GetStatusConditions(), corev1alpha1.ObjectDeploymentAvailable)
	if availCond == nil {
		return false
	}

	var (
		available   = availCond.Status == metav1.ConditionTrue
		noDelay     = objectSet.GetSpecSuccessDelaySeconds() == 0
		delayTarget = availCond.LastTransitionTime.Add(
			time.Duration(objectSet.GetSpecSuccessDelaySeconds() * int32(time.Second)),
		)
	)

	// noDelay avoids false negative for edgecase where objectSet
	// is available on first pass, but no delay is set
	return available && (noDelay || r.cfg.Clock.Now().After(delayTarget))
}

type objectSetPhasesReconcilerConfig struct {
	Clock clock
	controllers.BackoffConfig
}

func (c *objectSetPhasesReconcilerConfig) Option(opts ...objectSetPhasesReconcilerOption) {
	for _, opt := range opts {
		opt.ConfigureObjectSetPhasesReconciler(c)
	}
}

func (c *objectSetPhasesReconcilerConfig) Default() {
	if c.Clock == nil {
		c.Clock = defaultClock{}
	}

	c.BackoffConfig.Default()
}

type objectSetPhasesReconcilerOption interface {
	ConfigureObjectSetPhasesReconciler(*objectSetPhasesReconcilerConfig)
}

type clock interface {
	Now() time.Time
}

type defaultClock struct{}

func (c defaultClock) Now() time.Time {
	return time.Now()
}

func aggregateLocalObjects(objectSet adapters.ObjectSetAccessor) []client.Object {
	objectsInSet := []client.Object{}
	for _, phase := range objectSet.GetSpecPhases() {
		if phase.Class != "" {
			continue
		}
		for _, object := range phase.Objects {
			objectsInSet = append(objectsInSet, &object.Object)
		}
	}
	return objectsInSet
}

// Adds a RemotePhaseReference if it's not already part of the slice.
func addRemoteObjectSetPhase(
	refs []corev1alpha1.RemotePhaseReference,
	ref corev1alpha1.RemotePhaseReference,
) []corev1alpha1.RemotePhaseReference {
	for i := range refs {
		if refs[i].Name == ref.Name {
			refs[i] = ref
			return refs
		}
	}
	refs = append(refs, ref)
	return refs
}

func mapConditions(actualObjects []machinery.Object, owner adapters.ObjectSetAccessor) error {
	var ownerObjects []corev1alpha1.ObjectSetObject
	for _, phase := range owner.GetSpecPhases() {
		ownerObjects = append(ownerObjects, phase.Objects...)
	}

	return controllers.MapConditionsToOwner(actualObjects, ownerObjects, owner)
}
