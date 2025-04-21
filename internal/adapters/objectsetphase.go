package adapters

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type ObjectSetPhaseAccessor interface {
	ClientObject() client.Object
	GetConditions() *[]metav1.Condition
	GetClass() string
	GetPrevious() []corev1alpha1.PreviousRevisionReference
	SetPrevious([]corev1alpha1.PreviousRevisionReference)
	GetPhase() corev1alpha1.ObjectSetTemplatePhase
	SetPhase(corev1alpha1.ObjectSetTemplatePhase)
	GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe
	SetAvailabilityProbes([]corev1alpha1.ObjectSetProbe)
	GetRevision() int64
	SetRevision(int64)
	GetGeneration() int64
	IsSpecPaused() bool
	SetPaused(bool)
	SetStatusControllerOf([]corev1alpha1.ControlledObjectReference)
	GetStatusControllerOf() []corev1alpha1.ControlledObjectReference
}

var (
	_ ObjectSetPhaseAccessor = (*ObjectSetPhaseAdapter)(nil)
	_ ObjectSetPhaseAccessor = (*ClusterObjectSetPhaseAdapter)(nil)
)

type ObjectSetPhaseFactory func(scheme *runtime.Scheme) ObjectSetPhaseAccessor

var (
	objectSetPhaseGVK        = corev1alpha1.GroupVersion.WithKind("ObjectSetPhase")
	clusterObjectSetPhaseGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectSetPhase")
)

func NewObjectSetPhaseAccessor(scheme *runtime.Scheme) ObjectSetPhaseAccessor {
	obj, err := scheme.New(objectSetPhaseGVK)
	if err != nil {
		panic(err)
	}

	return &ObjectSetPhaseAdapter{
		ObjectSetPhase: *obj.(*corev1alpha1.ObjectSetPhase),
	}
}

func NewClusterObjectSetPhaseAccessor(scheme *runtime.Scheme) ObjectSetPhaseAccessor {
	obj, err := scheme.New(clusterObjectSetPhaseGVK)
	if err != nil {
		panic(err)
	}

	return &ClusterObjectSetPhaseAdapter{
		ClusterObjectSetPhase: *obj.(*corev1alpha1.ClusterObjectSetPhase),
	}
}

type ObjectSetPhaseAdapter struct {
	corev1alpha1.ObjectSetPhase
}

func (a *ObjectSetPhaseAdapter) ClientObject() client.Object {
	return &a.ObjectSetPhase
}

func (a *ObjectSetPhaseAdapter) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *ObjectSetPhaseAdapter) GetClass() string {
	return a.Labels[corev1alpha1.ObjectSetPhaseClassLabel]
}

func (a *ObjectSetPhaseAdapter) GetPrevious() []corev1alpha1.PreviousRevisionReference {
	return a.Spec.Previous
}

func (a *ObjectSetPhaseAdapter) SetPrevious(previous []corev1alpha1.PreviousRevisionReference) {
	a.Spec.Previous = previous
}

func (a *ObjectSetPhaseAdapter) GetPhase() corev1alpha1.ObjectSetTemplatePhase {
	return corev1alpha1.ObjectSetTemplatePhase{
		Objects: a.Spec.Objects,
	}
}

func (a *ObjectSetPhaseAdapter) SetPhase(phase corev1alpha1.ObjectSetTemplatePhase) {
	a.Spec.Objects = phase.Objects
}

func (a *ObjectSetPhaseAdapter) GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe {
	return a.Spec.AvailabilityProbes
}

func (a *ObjectSetPhaseAdapter) SetAvailabilityProbes(probes []corev1alpha1.ObjectSetProbe) {
	a.Spec.AvailabilityProbes = probes
}

func (a *ObjectSetPhaseAdapter) GetRevision() int64 {
	return a.Spec.Revision
}

func (a *ObjectSetPhaseAdapter) SetRevision(revision int64) {
	a.Spec.Revision = revision
}

func (a *ObjectSetPhaseAdapter) IsSpecPaused() bool {
	return a.Spec.Paused
}

func (a *ObjectSetPhaseAdapter) SetPaused(paused bool) {
	a.Spec.Paused = paused
}

func (a *ObjectSetPhaseAdapter) GetGeneration() int64 {
	return a.Generation
}

func (a *ObjectSetPhaseAdapter) SetStatusControllerOf(controllerOf []corev1alpha1.ControlledObjectReference) {
	a.Status.ControllerOf = controllerOf
}

func (a *ObjectSetPhaseAdapter) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	return a.Status.ControllerOf
}

type ClusterObjectSetPhaseAdapter struct {
	corev1alpha1.ClusterObjectSetPhase
}

func (a *ClusterObjectSetPhaseAdapter) ClientObject() client.Object {
	return &a.ClusterObjectSetPhase
}

func (a *ClusterObjectSetPhaseAdapter) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *ClusterObjectSetPhaseAdapter) GetClass() string {
	return a.Labels[corev1alpha1.ObjectSetPhaseClassLabel]
}

func (a *ClusterObjectSetPhaseAdapter) GetPrevious() []corev1alpha1.PreviousRevisionReference {
	return a.Spec.Previous
}

func (a *ClusterObjectSetPhaseAdapter) SetPrevious(previous []corev1alpha1.PreviousRevisionReference) {
	a.Spec.Previous = previous
}

func (a *ClusterObjectSetPhaseAdapter) GetPhase() corev1alpha1.ObjectSetTemplatePhase {
	return corev1alpha1.ObjectSetTemplatePhase{
		Objects: a.Spec.Objects,
	}
}

func (a *ClusterObjectSetPhaseAdapter) SetPhase(phase corev1alpha1.ObjectSetTemplatePhase) {
	a.Spec.Objects = phase.Objects
}

func (a *ClusterObjectSetPhaseAdapter) GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe {
	return a.Spec.AvailabilityProbes
}

func (a *ClusterObjectSetPhaseAdapter) SetAvailabilityProbes(probes []corev1alpha1.ObjectSetProbe) {
	a.Spec.AvailabilityProbes = probes
}

func (a *ClusterObjectSetPhaseAdapter) GetRevision() int64 {
	return a.Spec.Revision
}

func (a *ClusterObjectSetPhaseAdapter) SetRevision(revision int64) {
	a.Spec.Revision = revision
}

func (a *ClusterObjectSetPhaseAdapter) GetGeneration() int64 {
	return a.Generation
}

func (a *ClusterObjectSetPhaseAdapter) IsSpecPaused() bool {
	return a.Spec.Paused
}

func (a *ClusterObjectSetPhaseAdapter) SetPaused(paused bool) {
	a.Spec.Paused = paused
}

func (a *ClusterObjectSetPhaseAdapter) SetStatusControllerOf(controllerOf []corev1alpha1.ControlledObjectReference) {
	a.Status.ControllerOf = controllerOf
}

func (a *ClusterObjectSetPhaseAdapter) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	return a.Status.ControllerOf
}
