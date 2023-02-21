package adoption

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coordinationv1alpha1 "package-operator.run/apis/coordination/v1alpha1"
)

type genericAdoption interface {
	ClientObject() client.Object
	GetTargetAPI() coordinationv1alpha1.TargetAPI
	GetStrategyType() coordinationv1alpha1.AdoptionStrategyType
	GetStaticStrategy() coordinationv1alpha1.AdoptionStrategyStaticSpec
	GetRoundRobinSpec() coordinationv1alpha1.AdoptionStrategyRoundRobinSpec
	GetRoundRobinStatus() *coordinationv1alpha1.AdoptionRoundRobinStatus
	SetRoundRobinStatus(*coordinationv1alpha1.AdoptionRoundRobinStatus)
	GetConditions() *[]metav1.Condition
	UpdatePhase()
}

type genericAdoptionFactory func(scheme *runtime.Scheme) genericAdoption

var (
	adoptionGVK        = coordinationv1alpha1.GroupVersion.WithKind("Adoption")
	clusterAdoptionGVK = coordinationv1alpha1.GroupVersion.WithKind("ClusterAdoption")
)

func newGenericAdoption(scheme *runtime.Scheme) genericAdoption {
	obj, err := scheme.New(adoptionGVK)
	if err != nil {
		panic(err)
	}

	return &GenericAdoption{
		Adoption: *obj.(*coordinationv1alpha1.Adoption),
	}
}

func newGenericClusterAdoption(scheme *runtime.Scheme) genericAdoption {
	obj, err := scheme.New(clusterAdoptionGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterAdoption{
		ClusterAdoption: *obj.(*coordinationv1alpha1.ClusterAdoption),
	}
}

var (
	_ genericAdoption = (*GenericAdoption)(nil)
	_ genericAdoption = (*GenericClusterAdoption)(nil)
)

type GenericAdoption struct {
	coordinationv1alpha1.Adoption
}

func (a *GenericAdoption) ClientObject() client.Object {
	return &a.Adoption
}

func (a *GenericAdoption) GetTargetAPI() coordinationv1alpha1.TargetAPI {
	return a.Spec.TargetAPI
}

func (a *GenericAdoption) GetStrategyType() coordinationv1alpha1.AdoptionStrategyType {
	return a.Spec.Strategy.Type
}

func (a *GenericAdoption) GetStaticStrategy() coordinationv1alpha1.AdoptionStrategyStaticSpec {
	if a.Spec.Strategy.Static != nil {
		return *a.Spec.Strategy.Static
	}
	return coordinationv1alpha1.AdoptionStrategyStaticSpec{}
}

func (a *GenericAdoption) UpdatePhase() {
	if meta.IsStatusConditionTrue(
		a.Status.Conditions, coordinationv1alpha1.AdoptionActive) {
		a.Status.Phase = coordinationv1alpha1.AdoptionPhaseActive
	} else {
		a.Status.Phase = coordinationv1alpha1.AdoptionPhasePending
	}
}

func (a *GenericAdoption) GetRoundRobinSpec() coordinationv1alpha1.AdoptionStrategyRoundRobinSpec {
	return *a.Spec.Strategy.RoundRobin
}

func (a *GenericAdoption) GetRoundRobinStatus() *coordinationv1alpha1.AdoptionRoundRobinStatus {
	return a.Status.RoundRobin
}

func (a *GenericAdoption) SetRoundRobinStatus(rr *coordinationv1alpha1.AdoptionRoundRobinStatus) {
	a.Status.RoundRobin = rr
}

func (a *GenericAdoption) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

type GenericClusterAdoption struct {
	coordinationv1alpha1.ClusterAdoption
}

func (a *GenericClusterAdoption) ClientObject() client.Object {
	return &a.ClusterAdoption
}

func (a *GenericClusterAdoption) GetTargetAPI() coordinationv1alpha1.TargetAPI {
	return a.Spec.TargetAPI
}

func (a *GenericClusterAdoption) GetStrategyType() coordinationv1alpha1.AdoptionStrategyType {
	return a.Spec.Strategy.Type
}

func (a *GenericClusterAdoption) GetStaticStrategy() coordinationv1alpha1.AdoptionStrategyStaticSpec {
	if a.Spec.Strategy.Static != nil {
		return *a.Spec.Strategy.Static
	}
	return coordinationv1alpha1.AdoptionStrategyStaticSpec{}
}

func (a *GenericClusterAdoption) UpdatePhase() {
	if meta.IsStatusConditionTrue(
		a.Status.Conditions, coordinationv1alpha1.AdoptionActive) {
		a.Status.Phase = coordinationv1alpha1.AdoptionPhaseActive
	} else {
		a.Status.Phase = coordinationv1alpha1.AdoptionPhasePending
	}
}

func (a *GenericClusterAdoption) GetRoundRobinSpec() coordinationv1alpha1.AdoptionStrategyRoundRobinSpec {
	return *a.Spec.Strategy.RoundRobin
}

func (a *GenericClusterAdoption) GetRoundRobinStatus() *coordinationv1alpha1.AdoptionRoundRobinStatus {
	return a.Status.RoundRobin
}

func (a *GenericClusterAdoption) SetRoundRobinStatus(rr *coordinationv1alpha1.AdoptionRoundRobinStatus) {
	a.Status.RoundRobin = rr
}

func (a *GenericClusterAdoption) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}
