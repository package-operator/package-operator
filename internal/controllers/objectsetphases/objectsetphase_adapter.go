package objectsetphases

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type genericObjectSetPhase interface {
	ClientObject() client.Object
	GetConditions() *[]metav1.Condition
	GetClass() string
	GetPrevious() []corev1alpha1.PreviousRevisionReference
	GetPhase() corev1alpha1.ObjectSetTemplatePhase
	GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe
	GetRevision() int64
	GetGeneration() int64
	IsPaused() bool
	SetStatusControllerOf([]corev1alpha1.ControlledObjectReference)
	UpdateStatusPhase()
}

var (
	_ genericObjectSetPhase = (*GenericObjectSetPhase)(nil)
	_ genericObjectSetPhase = (*GenericClusterObjectSetPhase)(nil)
)

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
		ObjectSetPhase: *obj.(*corev1alpha1.ObjectSetPhase),
	}
}

func newGenericClusterObjectSetPhase(scheme *runtime.Scheme) genericObjectSetPhase {
	obj, err := scheme.New(clusterObjectSetPhaseGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterObjectSetPhase{
		ClusterObjectSetPhase: *obj.(*corev1alpha1.ClusterObjectSetPhase),
	}
}

type GenericObjectSetPhase struct {
	corev1alpha1.ObjectSetPhase
}

func (a *GenericObjectSetPhase) ClientObject() client.Object {
	return &a.ObjectSetPhase
}

func (a *GenericObjectSetPhase) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericObjectSetPhase) GetClass() string {
	return a.Labels[corev1alpha1.ObjectSetPhaseClassLabel]
}

func (a *GenericObjectSetPhase) GetPrevious() []corev1alpha1.PreviousRevisionReference {
	return a.Spec.Previous
}

func (a *GenericObjectSetPhase) GetPhase() corev1alpha1.ObjectSetTemplatePhase {
	return corev1alpha1.ObjectSetTemplatePhase{
		Objects: a.Spec.Objects,
	}
}

func (a *GenericObjectSetPhase) GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe {
	return a.Spec.AvailabilityProbes
}

func (a *GenericObjectSetPhase) GetRevision() int64 {
	return a.Spec.Revision
}

func (a *GenericObjectSetPhase) IsPaused() bool {
	return a.Spec.Paused
}

func (a *GenericObjectSetPhase) GetGeneration() int64 {
	return a.Generation
}
func (a *GenericObjectSetPhase) UpdateStatusPhase() {}

func (a *GenericObjectSetPhase) SetStatusControllerOf(controllerOf []corev1alpha1.ControlledObjectReference) {
	a.Status.ControllerOf = controllerOf
}

type GenericClusterObjectSetPhase struct {
	corev1alpha1.ClusterObjectSetPhase
}

func (a *GenericClusterObjectSetPhase) ClientObject() client.Object {
	return &a.ClusterObjectSetPhase
}

func (a *GenericClusterObjectSetPhase) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericClusterObjectSetPhase) GetClass() string {
	return a.Labels[corev1alpha1.ObjectSetPhaseClassLabel]
}

func (a *GenericClusterObjectSetPhase) GetPrevious() []corev1alpha1.PreviousRevisionReference {
	return a.Spec.Previous
}

func (a *GenericClusterObjectSetPhase) GetPhase() corev1alpha1.ObjectSetTemplatePhase {
	return corev1alpha1.ObjectSetTemplatePhase{
		Objects: a.Spec.Objects,
	}
}

func (a *GenericClusterObjectSetPhase) GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe {
	return a.Spec.AvailabilityProbes
}

func (a *GenericClusterObjectSetPhase) GetRevision() int64 {
	return a.Spec.Revision
}

func (a *GenericClusterObjectSetPhase) GetGeneration() int64 {
	return a.Generation
}

func (a *GenericClusterObjectSetPhase) IsPaused() bool {
	return a.Spec.Paused
}

func (a *GenericClusterObjectSetPhase) SetStatusControllerOf(controllerOf []corev1alpha1.ControlledObjectReference) {
	a.Status.ControllerOf = controllerOf
}
func (a *GenericClusterObjectSetPhase) UpdateStatusPhase() {}
