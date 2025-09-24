package boxcutterutil

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"pkg.package-operator.run/boxcutter"
	"pkg.package-operator.run/boxcutter/machinery"
	"pkg.package-operator.run/boxcutter/machinery/types"
	"pkg.package-operator.run/boxcutter/managedcache"
	"pkg.package-operator.run/boxcutter/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"package-operator.run/internal/constants"
)

type PhaseEngine interface {
	Reconcile(
		ctx context.Context,
		owner client.Object,
		revision int64,
		phase types.Phase,
		opts ...types.PhaseReconcileOption,
	) (machinery.PhaseResult, error)
	Teardown(
		ctx context.Context,
		owner client.Object,
		revision int64,
		phase types.Phase,
		opts ...types.PhaseTeardownOption,
	) (machinery.PhaseTeardownResult, error)
}

type PhaseEngineFactory interface {
	New(managedcache.Accessor) (PhaseEngine, error)
}

type phaseEngineFactory struct {
	scheme          *runtime.Scheme
	discoveryClient discovery.DiscoveryInterface
	restMapper      meta.RESTMapper
	ownerStrategy   boxcutter.OwnerStrategy
	phaseValidator  *validation.PhaseValidator
}

type OwnerStrategy interface {
	SetOwnerReference(owner, obj metav1.Object) error
	SetControllerReference(owner, obj metav1.Object) error
	GetController(obj metav1.Object) (metav1.OwnerReference, bool)
	IsController(owner, obj metav1.Object) bool
	CopyOwnerReferences(objA, objB metav1.Object)
	EnqueueRequestForOwner(ownerType client.Object, mapper meta.RESTMapper, isController bool) handler.EventHandler
	ReleaseController(obj metav1.Object)
	RemoveOwner(owner, obj metav1.Object)
	IsOwner(owner, obj metav1.Object) bool
}

func NewPhaseEngineFactory(
	scheme *runtime.Scheme,
	discoveryClient discovery.DiscoveryInterface,
	restMapper meta.RESTMapper,
	ownerStrategy OwnerStrategy,
	phaseValidator *validation.PhaseValidator,
) PhaseEngineFactory {
	return phaseEngineFactory{
		scheme:          scheme,
		discoveryClient: discoveryClient,
		restMapper:      restMapper,
		ownerStrategy:   ownerStrategy,
		phaseValidator:  phaseValidator,
	}
}

func (f phaseEngineFactory) New(accessor managedcache.Accessor) (PhaseEngine, error) {
	pe, err := boxcutter.NewPhaseEngine(boxcutter.RevisionEngineOptions{
		Scheme:          f.scheme,
		FieldOwner:      constants.FieldOwner,
		SystemPrefix:    constants.SystemPrefix,
		DiscoveryClient: f.discoveryClient,
		RestMapper:      f.restMapper,
		Writer:          accessor,
		Reader:          accessor,
		OwnerStrategy:   f.ownerStrategy,
		PhaseValidator:  f.phaseValidator,
	})
	if err != nil {
		return nil, err
	}
	return pe, nil
}
