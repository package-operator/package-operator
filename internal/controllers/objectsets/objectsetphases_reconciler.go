package objectsets

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/flowcontrol"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"pkg.package-operator.run/boxcutter/probing"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/preflight"
	internalprobing "package-operator.run/internal/probing"

	"pkg.package-operator.run/boxcutter/managedcache"
	"pkg.package-operator.run/boxcutter/ownerhandling"
)

// objectSetPhasesReconciler reconciles all phases within an ObjectSet.
type objectSetPhasesReconciler struct {
	cfg                     objectSetPhasesReconcilerConfig
	scheme                  *runtime.Scheme
	accessManager           managedcache.ObjectBoundAccessManager[client.Object]
	phaseReconcilerFactory  controllers.PhaseReconcilerFactory
	remotePhase             remotePhaseReconciler
	lookupPreviousRevisions lookupPreviousRevisions
	ownerStrategy           ownerStrategy
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
	phaseReconcilerFactory controllers.PhaseReconcilerFactory,
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
		phaseReconcilerFactory:  phaseReconcilerFactory,
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
	) ([]corev1alpha1.ControlledObjectReference, controllers.ProbingResult, error)
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

	controllerOf, probingResult, err := r.reconcile(ctx, objectSet)
	if controllers.IsExternalResourceNotFound(err) {
		id := string(objectSet.ClientObject().GetUID())

		r.backoff.Next(id, r.backoff.Clock.Now())

		return ctrl.Result{
			RequeueAfter: r.backoff.Get(id),
		}, nil
	} else if err != nil {
		return res, err
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

	if !probingResult.IsZero() {
		meta.SetStatusCondition(objectSet.GetStatusConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetAvailable,
			Status:             metav1.ConditionFalse,
			Reason:             "ProbeFailure",
			Message:            probingResult.String(),
			ObservedGeneration: objectSet.ClientObject().GetGeneration(),
		})

		return res, nil
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

	return
}

func (r *objectSetPhasesReconciler) reconcile(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
) ([]corev1alpha1.ControlledObjectReference, controllers.ProbingResult, error) {
	log := logr.FromContextOrDiscard(ctx).WithName("objectSetPhasesReconciler")

	previous, err := r.lookupPreviousRevisions(ctx, objectSet)
	if err != nil {
		return nil, controllers.ProbingResult{}, fmt.Errorf("lookup previous revisions: %w", err)
	}

	probe, err := internalprobing.Parse(
		ctx, objectSet.GetAvailabilityProbes())
	if err != nil {
		return nil, controllers.ProbingResult{}, fmt.Errorf("parsing probes: %w", err)
	}

	log.Info("getting cache accessor")
	cache, err := r.accessManager.GetWithUser(
		ctx,
		constants.StaticCacheOwner(),
		objectSet.ClientObject(),
		aggregateLocalObjects(objectSet),
	)
	if err != nil {
		return nil, controllers.ProbingResult{}, fmt.Errorf("getting cache: %w", err)
	}

	log.Info("getting phaseReconciler")
	phaseReconciler := r.phaseReconcilerFactory.New(cache)

	var controllerOfAll []corev1alpha1.ControlledObjectReference
	for _, phase := range objectSet.GetSpecPhases() {
		log.Info("reconciling phase", "name", phase.Name, "class", phase.Class)

		controllerOf, probingResult, err := r.reconcilePhase(ctx, phaseReconciler, objectSet, phase, probe, previous)
		if err != nil {
			return nil, controllers.ProbingResult{}, err
		}

		// always gather all objects we are controller of
		controllerOfAll = append(controllerOfAll, controllerOf...)

		if !probingResult.IsZero() {
			// break on first failing probe
			return controllerOfAll, probingResult, nil
		}
	}

	return controllerOfAll, controllers.ProbingResult{}, nil
}

func (r *objectSetPhasesReconciler) reconcilePhase(
	ctx context.Context,
	phaseReconciler controllers.PhaseReconciler,
	objectSet adapters.ObjectSetAccessor,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober,
	previous []controllers.PreviousObjectSet,
) ([]corev1alpha1.ControlledObjectReference, controllers.ProbingResult, error) {
	if len(phase.Class) > 0 {
		return r.remotePhase.Reconcile(
			ctx, objectSet, phase)
	}
	return r.reconcileLocalPhase(
		ctx, phaseReconciler, objectSet, phase, probe, previous)
}

// Reconciles the Phase directly in-process.
func (r *objectSetPhasesReconciler) reconcileLocalPhase(
	ctx context.Context,
	phaseReconciler controllers.PhaseReconciler,
	objectSet adapters.ObjectSetAccessor,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober,
	previous []controllers.PreviousObjectSet,
) ([]corev1alpha1.ControlledObjectReference, controllers.ProbingResult, error) {
	actualObjects, probingResult, err := phaseReconciler.ReconcilePhase(
		ctx, objectSet, phase, probe, previous)
	if err != nil {
		return nil, probingResult, err
	}

	controllerOf, err := controllers.GetStatusControllerOf(
		ctx, r.scheme, r.ownerStrategy,
		objectSet.ClientObject(), actualObjects)
	if err != nil {
		return nil, controllers.ProbingResult{}, err
	}
	return controllerOf, probingResult, nil
}

func (r *objectSetPhasesReconciler) Teardown(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
) (cleanupDone bool, err error) {
	log := logr.FromContextOrDiscard(ctx)

	// objectSet is deleted with the `orphan` cascade option, so we don't delete the owned objects
	if controllerutil.ContainsFinalizer(objectSet.ClientObject(), "orphan") {
		return true, nil
	}

	phases := objectSet.GetSpecPhases()
	reverse(phases) // teardown in reverse order

	cache, err := r.accessManager.GetWithUser(
		ctx,
		constants.StaticCacheOwner(),
		objectSet.ClientObject(),
		aggregateLocalObjects(objectSet),
	)
	if err != nil {
		return false, fmt.Errorf("getting cache: %w", err)
	}

	phaseReconciler := r.phaseReconcilerFactory.New(cache)

	for _, phase := range phases {
		if cleanupDone, err := r.teardownPhase(ctx, phaseReconciler, objectSet, phase); err != nil {
			return false, fmt.Errorf("error archiving phase: %w", err)
		} else if !cleanupDone {
			return false, nil
		}
		log.Info("cleanup done", "phase", phase.Name)
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

func (r *objectSetPhasesReconciler) teardownPhase(
	ctx context.Context,
	phaseReconciler controllers.PhaseReconciler,
	objectSet adapters.ObjectSetAccessor,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	if len(phase.Class) > 0 {
		return r.remotePhase.Teardown(ctx, objectSet, phase)
	}
	return phaseReconciler.TeardownPhase(ctx, objectSet, phase)
}

// reverse the order of a slice.
func reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
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

type withClock struct {
	Clock clock
}

func (w withClock) ConfigureObjectSetPhasesReconciler(c *objectSetPhasesReconcilerConfig) {
	c.Clock = w.Clock
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
