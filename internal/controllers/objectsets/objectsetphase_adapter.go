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
	IsPaused() bool
	SetPhase(phase corev1alpha1.ObjectSetTemplatePhase)
	SetPaused(paused bool)
	SetAvailabilityProbes([]corev1alpha1.ObjectSetProbe)
	SetRevision(revision int64)
	SetPrevious([]corev1alpha1.PreviousRevisionReference)
	GetStatusControllerOf() []corev1alpha1.ControlledObjectReference
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

func (a *GenericObjectSetPhase) SetPaused(paused bool) {
	a.Spec.Paused = paused
}

func (a *GenericObjectSetPhase) SetAvailabilityProbes(probes []corev1alpha1.ObjectSetProbe) {
	a.Spec.AvailabilityProbes = probes
}

func (a *GenericObjectSetPhase) IsPaused() bool {
	return a.Spec.Paused
}

func (a *GenericObjectSetPhase) SetPhase(phase corev1alpha1.ObjectSetTemplatePhase) {
	if a.Labels == nil {
		a.Labels = map[string]string{}
	}
	a.Labels[corev1alpha1.ObjectSetPhaseClassLabel] = phase.Class
	a.Spec.Objects = phase.Objects
}

func (a *GenericObjectSetPhase) SetRevision(revision int64) {
	a.Spec.Revision = revision
}

func (a *GenericObjectSetPhase) SetPrevious(previous []corev1alpha1.PreviousRevisionReference) {
	a.Spec.Previous = previous
}

func (a *GenericObjectSetPhase) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	return a.Status.ControllerOf
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

func (a *GenericClusterObjectSetPhase) SetPaused(paused bool) {
	a.Spec.Paused = paused
}

func (a *GenericClusterObjectSetPhase) IsPaused() bool {
	return a.Spec.Paused
}

func (a *GenericClusterObjectSetPhase) SetAvailabilityProbes(probes []corev1alpha1.ObjectSetProbe) {
	a.Spec.AvailabilityProbes = probes
}

func (a *GenericClusterObjectSetPhase) SetPhase(phase corev1alpha1.ObjectSetTemplatePhase) {
	if a.Labels == nil {
		a.Labels = map[string]string{}
	}
	a.Labels[corev1alpha1.ObjectSetPhaseClassLabel] = phase.Class
	a.Spec.Objects = phase.Objects
}

func (a *GenericClusterObjectSetPhase) SetRevision(revision int64) {
	a.Spec.Revision = revision
}

func (a *GenericClusterObjectSetPhase) SetPrevious(previous []corev1alpha1.PreviousRevisionReference) {
	a.Spec.Previous = previous
}

func (a *GenericClusterObjectSetPhase) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	return a.Status.ControllerOf
}
