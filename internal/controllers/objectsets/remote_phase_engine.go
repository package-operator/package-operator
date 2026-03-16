package objectsets

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"pkg.package-operator.run/boxcutter/machinery"
	"pkg.package-operator.run/boxcutter/machinery/types"
	"pkg.package-operator.run/boxcutter/managedcache"
	"pkg.package-operator.run/boxcutter/validation"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/controllers/boxcutterutil"
)

var (
	_ boxcutterutil.PhaseEngine        = (*remoteEnabledPhaseEngine)(nil)
	_ boxcutterutil.PhaseEngineFactory = (*remoteEnabledPhaseEngineFactory)(nil)
	_ machinery.PhaseResult            = (*remotePhaseResult)(nil)
	_ machinery.PhaseTeardownResult    = (*remotePhaseTeardownResult)(nil)
)

type remoteEnabledPhaseEngineFactory struct {
	phaseEngineFactory    boxcutterutil.PhaseEngineFactory
	remotePhaseReconciler remotePhaseReconciler
}

func newRemoteEnabledPhaseEngineFactory(
	scheme *runtime.Scheme,
	discoveryClient boxcutterutil.DiscoveryClient,
	restMapper meta.RESTMapper,
	phaseValidator *validation.PhaseValidator,
	remotePhaseReconciler remotePhaseReconciler,
) boxcutterutil.PhaseEngineFactory {
	return remoteEnabledPhaseEngineFactory{
		phaseEngineFactory: boxcutterutil.NewPhaseEngineFactory(
			scheme, discoveryClient, restMapper, phaseValidator),
		remotePhaseReconciler: remotePhaseReconciler,
	}
}

func (f remoteEnabledPhaseEngineFactory) New(accessor managedcache.Accessor) (boxcutterutil.PhaseEngine, error) {
	pe, err := f.phaseEngineFactory.New(accessor)
	if err != nil {
		return nil, err
	}
	return &remoteEnabledPhaseEngine{
		pe:                    pe,
		remotePhaseReconciler: f.remotePhaseReconciler,
	}, nil
}

type remoteEnabledPhaseEngine struct {
	pe                    boxcutterutil.PhaseEngine
	remotePhaseReconciler remotePhaseReconciler
}

func (r *remoteEnabledPhaseEngine) Reconcile(
	ctx context.Context, revision int64,
	phase types.Phase, opts ...types.PhaseReconcileOption,
) (machinery.PhaseResult, error) {
	owner := getReconcileOwner(phase, opts...)
	if hasClass(owner, phase) {
		return r.reconcileRemotePhase(ctx, owner, phase)
	}
	return r.pe.Reconcile(ctx, revision, phase, opts...)
}

func (r *remoteEnabledPhaseEngine) Teardown(
	ctx context.Context, revision int64,
	phase types.Phase, opts ...types.PhaseTeardownOption,
) (machinery.PhaseTeardownResult, error) {
	owner := getTeardownOwner(phase, opts...)
	if hasClass(owner, phase) {
		return r.teardownRemotePhase(ctx, owner, phase)
	}
	return r.pe.Teardown(ctx, revision, phase, opts...)
}

func getReconcileOwner(phase types.Phase, opts ...types.PhaseReconcileOption) adapters.ObjectSetAccessor {
	var phaseOptions types.PhaseReconcileOptions
	for _, opt := range opts {
		opt.ApplyToPhaseReconcileOptions(&phaseOptions)
	}

	objs := phase.GetObjects()
	if len(objs) == 0 {
		return nil
	}

	var objOptions types.ObjectReconcileOptions
	for _, opt := range phaseOptions.ForObject(objs[0]) {
		opt.ApplyToObjectReconcileOptions(&objOptions)
	}

	switch o := objOptions.Owner.(type) {
	case *corev1alpha1.ObjectSet:
		return &adapters.ObjectSetAdapter{ObjectSet: *o}
	case *corev1alpha1.ClusterObjectSet:
		return &adapters.ClusterObjectSetAdapter{ClusterObjectSet: *o}
	}
	return nil
}

func getTeardownOwner(phase types.Phase, opts ...types.PhaseTeardownOption) adapters.ObjectSetAccessor {
	var phaseOptions types.PhaseTeardownOptions
	for _, opt := range opts {
		opt.ApplyToPhaseTeardownOptions(&phaseOptions)
	}

	objs := phase.GetObjects()
	if len(objs) == 0 {
		return nil
	}

	var objOptions types.ObjectTeardownOptions
	for _, opt := range phaseOptions.ForObject(objs[0]) {
		opt.ApplyToObjectTeardownOptions(&objOptions)
	}

	switch o := objOptions.Owner.(type) {
	case *corev1alpha1.ObjectSet:
		return &adapters.ObjectSetAdapter{ObjectSet: *o}
	case *corev1alpha1.ClusterObjectSet:
		return &adapters.ClusterObjectSetAdapter{ClusterObjectSet: *o}
	}
	return nil
}

func (r *remoteEnabledPhaseEngine) reconcileRemotePhase(
	ctx context.Context, owner adapters.ObjectSetAccessor,
	boxcutterPhase types.Phase,
) (machinery.PhaseResult, error) {
	var phase corev1alpha1.ObjectSetTemplatePhase
	for _, p := range owner.GetSpecPhases() {
		if p.Name == boxcutterPhase.GetName() {
			phase = p
		}
	}

	controllerOf, probeResult, err := r.remotePhaseReconciler.Reconcile(ctx, owner, phase)
	if err != nil {
		return nil, err
	}

	result := &remotePhaseResult{
		name:         phase.Name,
		failedProbes: probeResult.FailedProbes,
		controllerOf: controllerOf,
	}
	return result, nil
}

func (r *remoteEnabledPhaseEngine) teardownRemotePhase(
	ctx context.Context, owner adapters.ObjectSetAccessor,
	boxcutterPhase types.Phase,
) (machinery.PhaseTeardownResult, error) {
	var phase corev1alpha1.ObjectSetTemplatePhase
	for _, p := range owner.GetSpecPhases() {
		if p.Name == boxcutterPhase.GetName() {
			phase = p
		}
	}

	cleanupDone, err := r.remotePhaseReconciler.Teardown(ctx, owner, phase)
	if err != nil {
		return nil, err
	}

	result := &remotePhaseTeardownResult{
		name:        phase.Name,
		cleanupDone: cleanupDone,
	}
	return result, nil
}

func hasClass(owner adapters.ObjectSetAccessor, phase types.Phase) bool {
	for _, p := range owner.GetSpecPhases() {
		if p.Name == phase.GetName() && len(p.Class) > 0 {
			return true
		}
	}
	return false
}

type remotePhaseResult struct {
	name         string
	failedProbes []string
	controllerOf []corev1alpha1.ControlledObjectReference
}

func (r remotePhaseResult) GetName() string {
	return r.name
}

func (r remotePhaseResult) GetValidationError() *validation.PhaseValidationError {
	if len(r.failedProbes) == 0 {
		return nil
	}
	return &validation.PhaseValidationError{
		PhaseName:  r.name,
		PhaseError: errors.New(r.failedProbes[0]),
	}
}

func (r remotePhaseResult) GetObjects() []machinery.ObjectResult {
	// TODO
	return nil
}

func (r remotePhaseResult) GetControllerOf() []corev1alpha1.ControlledObjectReference {
	return r.controllerOf
}

func (r remotePhaseResult) InTransition() bool {
	return len(r.failedProbes) > 0
}

func (r remotePhaseResult) IsComplete() bool {
	return r.failedProbes == nil
}

func (r remotePhaseResult) HasProgressed() bool {
	// TODO: add "no status" probe fail?
	return r.failedProbes == nil
}

func (r remotePhaseResult) String() string {
	var out strings.Builder
	fmt.Fprintf(&out,
		"Phase %q\nComplete: %t\nIn Transition: %t\n",
		r.name, r.IsComplete(), r.InTransition(),
	)

	if err := r.GetValidationError(); err != nil {
		fmt.Fprintln(&out, "Validation Errors:")

		for _, err := range err.Unwrap() {
			fmt.Fprintf(&out, "- %s\n", err.Error())
		}
	}

	fmt.Fprintln(&out, "ControllerOf:")

	for _, ref := range r.controllerOf {
		fmt.Fprintf(&out, "- %#v\n", ref)
	}

	return out.String()
}

type remotePhaseTeardownResult struct {
	name        string
	cleanupDone bool
}

func (r *remotePhaseTeardownResult) String() string {
	var out strings.Builder
	fmt.Fprintf(&out, "Phase %q\n", r.name)
	return out.String()
}

func (r *remotePhaseTeardownResult) GetName() string {
	return r.name
}

func (r *remotePhaseTeardownResult) IsComplete() bool {
	return r.cleanupDone
}

func (r *remotePhaseTeardownResult) Gone() []types.ObjectRef {
	// TODO
	return nil
}

func (r *remotePhaseTeardownResult) Waiting() []types.ObjectRef {
	// TODO
	return nil
}
