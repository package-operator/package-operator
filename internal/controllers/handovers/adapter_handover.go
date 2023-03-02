package handovers

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coordinationv1alpha1 "package-operator.run/apis/coordination/v1alpha1"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type genericHandover interface {
	ClientObject() client.Object
	GetTargetAPI() coordinationv1alpha1.TargetAPI
	GetStrategyType() coordinationv1alpha1.HandoverStrategyType
	GetRelabelStrategy() coordinationv1alpha1.HandoverStrategyRelabelSpec
	GetPartitionSpec() *coordinationv1alpha1.PartitionSpec
	GetAvailabilityProbes() []corev1alpha1.Probe
	GetProcessing() []coordinationv1alpha1.HandoverRefStatus
	SetProcessing(processing []coordinationv1alpha1.HandoverRefStatus)
	SetStats(total coordinationv1alpha1.HandoverCountsStatus, partitions []coordinationv1alpha1.HandoverPartitionStatus)
	GetConditions() *[]metav1.Condition
	UpdatePhase()
}

type genericHandoverFactory func(scheme *runtime.Scheme) genericHandover

var (
	clusterHandoverGVK = coordinationv1alpha1.GroupVersion.WithKind("ClusterHandover")
)

func newGenericClusterHandover(scheme *runtime.Scheme) genericHandover {
	obj, err := scheme.New(clusterHandoverGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterHandover{
		ClusterHandover: *obj.(*coordinationv1alpha1.ClusterHandover),
	}
}

var (
	_ genericHandover = (*GenericClusterHandover)(nil)
)

type GenericClusterHandover struct {
	coordinationv1alpha1.ClusterHandover
}

func (a *GenericClusterHandover) ClientObject() client.Object {
	return &a.ClusterHandover
}

func (a *GenericClusterHandover) GetTargetAPI() coordinationv1alpha1.TargetAPI {
	return a.Spec.TargetAPI
}

func (a *GenericClusterHandover) GetStrategyType() coordinationv1alpha1.HandoverStrategyType {
	return a.Spec.Strategy.Type
}

func (a *GenericClusterHandover) GetPartitionSpec() *coordinationv1alpha1.PartitionSpec {
	return a.Spec.Partition
}

func (a *GenericClusterHandover) GetRelabelStrategy() coordinationv1alpha1.HandoverStrategyRelabelSpec {
	if a.Spec.Strategy.Relabel != nil {
		return *a.Spec.Strategy.Relabel
	}
	return coordinationv1alpha1.HandoverStrategyRelabelSpec{}
}

func (a *GenericClusterHandover) GetAvailabilityProbes() []corev1alpha1.Probe {
	return a.Spec.AvailabilityProbes
}

func (a *GenericClusterHandover) GetProcessing() []coordinationv1alpha1.HandoverRefStatus {
	return a.Status.Processing
}

func (a *GenericClusterHandover) SetProcessing(processing []coordinationv1alpha1.HandoverRefStatus) {
	a.Status.Processing = processing
}

func (a *GenericClusterHandover) SetStats(total coordinationv1alpha1.HandoverCountsStatus, partitions []coordinationv1alpha1.HandoverPartitionStatus) {
	a.Status.Total = total
	a.Status.Partitions = partitions
}

func (a *GenericClusterHandover) UpdatePhase() {
	if meta.IsStatusConditionTrue(
		a.Status.Conditions, coordinationv1alpha1.HandoverCompleted) {
		a.Status.Phase = coordinationv1alpha1.HandoverPhaseCompleted
	} else {
		a.Status.Phase = coordinationv1alpha1.HandoverPhaseProgressing
	}
}

func (a *GenericClusterHandover) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}
