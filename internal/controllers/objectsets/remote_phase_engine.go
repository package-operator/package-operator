package objectsets

import (
	"context"
	"fmt"

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
	if owner != nil && hasClass(owner, phase) {
		pa, ok := phase.(*adapters.PhaseAdapter)
		if !ok {
			return nil, fmt.Errorf("remote phase %q is not backed by PhaseAdapter", phase.GetName())
		}
		return r.reconcileRemotePhase(ctx, pa.GetObjectSet(), phase)
	}
	return r.pe.Reconcile(ctx, revision, phase, opts...)
}

func (r *remoteEnabledPhaseEngine) Teardown(
	ctx context.Context, revision int64,
	phase types.Phase, opts ...types.PhaseTeardownOption,
) (machinery.PhaseTeardownResult, error) {
	owner := getTeardownOwner(phase, opts...)
	if owner != nil && hasClass(owner, phase) {
		pa, ok := phase.(*adapters.PhaseAdapter)
		if !ok {
			return nil, fmt.Errorf("remote phase %q is not backed by PhaseAdapter", phase.GetName())
		}
		return r.teardownRemotePhase(ctx, pa.GetObjectSet(), phase)
	}
	return r.pe.Teardown(ctx, revision, phase, opts...)
}

func ownerFromObjectOwner(objOwner any) adapters.ObjectSetAccessor {
	switch o := objOwner.(type) {
	case *corev1alpha1.ObjectSet:
		return &adapters.ObjectSetAdapter{ObjectSet: *o}
	case *corev1alpha1.ClusterObjectSet:
		return &adapters.ClusterObjectSetAdapter{ClusterObjectSet: *o}
	}
	return nil
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

	return ownerFromObjectOwner(objOptions.Owner)
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

	return ownerFromObjectOwner(objOptions.Owner)
}

func findSpecPhase(
	owner adapters.ObjectSetAccessor, phaseName string,
) (corev1alpha1.ObjectSetTemplatePhase, bool) {
	for _, p := range owner.GetSpecPhases() {
		if p.Name == phaseName {
			return p, true
		}
	}
	return corev1alpha1.ObjectSetTemplatePhase{}, false
}

func (r *remoteEnabledPhaseEngine) reconcileRemotePhase(
	ctx context.Context, owner adapters.ObjectSetAccessor,
	boxcutterPhase types.Phase,
) (machinery.PhaseResult, error) {
	phase, ok := findSpecPhase(owner, boxcutterPhase.GetName())
	if !ok {
		return nil, fmt.Errorf("phase %q not found in owner spec", boxcutterPhase.GetName())
	}

	return r.remotePhaseReconciler.Reconcile(ctx, owner, phase)
}

func (r *remoteEnabledPhaseEngine) teardownRemotePhase(
	ctx context.Context, owner adapters.ObjectSetAccessor,
	boxcutterPhase types.Phase,
) (machinery.PhaseTeardownResult, error) {
	phase, ok := findSpecPhase(owner, boxcutterPhase.GetName())
	if !ok {
		return nil, fmt.Errorf("phase %q not found in owner spec", boxcutterPhase.GetName())
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
	if owner == nil {
		return false
	}
	for _, p := range owner.GetSpecPhases() {
		if p.Name == phase.GetName() && len(p.Class) > 0 {
			return true
		}
	}
	return false
}
