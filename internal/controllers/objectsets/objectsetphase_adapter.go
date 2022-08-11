package objectsets

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type genericObjectSetPhase interface {
	ClientObject() client.Object
	GetConditions() []metav1.Condition
}

type genericObjectSetPhaseFactory func(
	scheme *runtime.Scheme) genericObjectSetPhase

var (
	objectSetPhaseGVK        = corev1alpha1.GroupVersion.WithKind("ObjectSetPhase")
	clusterObjectSetPhaseGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectSetPhase")
)

func newGenericObjectSetPhase(scheme *runtime.Scheme) genericObjectSetPhase {
	obj, err := scheme.New(objectSetPhaseGVK)
	if err != nil {
		panic(err)
	}

	return &GenericObjectSetPhase{
		ObjectSetPhase: *obj.(*corev1alpha1.ObjectSetPhase)}
}

func newGenericClusterObjectSetPhase(scheme *runtime.Scheme) genericObjectSetPhase {
	obj, err := scheme.New(clusterObjectSetPhaseGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterObjectSetPhase{
		ClusterObjectSetPhase: *obj.(*corev1alpha1.ClusterObjectSetPhase)}
}

var (
	_ genericObjectSetPhase = (*GenericObjectSetPhase)(nil)
	_ genericObjectSetPhase = (*GenericClusterObjectSetPhase)(nil)
)

type GenericObjectSetPhase struct {
	corev1alpha1.ObjectSetPhase
}

func (a *GenericObjectSetPhase) ClientObject() client.Object {
	return &a.ObjectSetPhase
}

func (a *GenericObjectSetPhase) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

type GenericClusterObjectSetPhase struct {
	corev1alpha1.ClusterObjectSetPhase
}

func (a *GenericClusterObjectSetPhase) ClientObject() client.Object {
	return &a.ClusterObjectSetPhase
}

func (a *GenericClusterObjectSetPhase) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}
