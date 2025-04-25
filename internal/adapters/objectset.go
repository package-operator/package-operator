package adapters

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

const (
	pausedByParentAnnotation = "package-operator.run/paused-by-parent"
	pausedByParentTrue       = "true"
)

type ObjectSetAccessor interface {
	client.Object
	ClientObject() client.Object
	GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec
	SetTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec)
	GetPrevious() []corev1alpha1.PreviousRevisionReference
	SetPreviousRevisions(prev []ObjectSetAccessor)
	GetPhases() []corev1alpha1.ObjectSetTemplatePhase
	SetPhases(phases []corev1alpha1.ObjectSetTemplatePhase)
	GetConditions() *[]metav1.Condition
	GetRevision() int64
	SetRevision(revision int64)
	GetGeneration() int64
	IsArchived() bool
	SetArchived()
	IsStatusPaused() bool
	IsSpecPaused() bool
	SetPaused()
	IsAvailable() bool
	GetPausedByParent() bool
	SetPausedByParent()
	SetActiveByParent()
	GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe
	GetSuccessDelaySeconds() int32
	GetRemotePhases() []corev1alpha1.RemotePhaseReference
	SetRemotePhases([]corev1alpha1.RemotePhaseReference)
	GetStatusControllerOf() []corev1alpha1.ControlledObjectReference
	SetStatusControllerOf([]corev1alpha1.ControlledObjectReference)
}

type ObjectSetAccessorFactory func(scheme *runtime.Scheme) ObjectSetAccessor

var (
	objectSetGVK        = corev1alpha1.GroupVersion.WithKind("ObjectSet")
	clusterObjectSetGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectSet")
)

func NewObjectSet(scheme *runtime.Scheme) ObjectSetAccessor {
	obj, err := scheme.New(objectSetGVK)
	if err != nil {
		panic(err)
	}

	return &ObjectSetAdapter{
		ObjectSet: *obj.(*corev1alpha1.ObjectSet),
	}
}

func NewClusterObjectSet(scheme *runtime.Scheme) ObjectSetAccessor {
	obj, err := scheme.New(clusterObjectSetGVK)
	if err != nil {
		panic(err)
	}

	return &ClusterObjectSetAdapter{
		ClusterObjectSet: *obj.(*corev1alpha1.ClusterObjectSet),
	}
}

var (
	_ ObjectSetAccessor = (*ObjectSetAdapter)(nil)
	_ ObjectSetAccessor = (*ClusterObjectSetAdapter)(nil)
)

type ObjectSetAdapter struct {
	corev1alpha1.ObjectSet
}

func (a *ObjectSetAdapter) ClientObject() client.Object {
	return &a.ObjectSet
}

func (a *ObjectSetAdapter) GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	return a.Spec.ObjectSetTemplateSpec
}

func (a *ObjectSetAdapter) SetTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec) {
	a.Spec.ObjectSetTemplateSpec = templateSpec
}

func (a *ObjectSetAdapter) GetPrevious() []corev1alpha1.PreviousRevisionReference {
	return a.Spec.Previous
}

func (a *ObjectSetAdapter) SetPreviousRevisions(prevObjectSets []ObjectSetAccessor) {
	a.Spec.Previous = make([]corev1alpha1.PreviousRevisionReference, len(prevObjectSets))
	for i := range prevObjectSets {
		a.Spec.Previous[i].Name = prevObjectSets[i].ClientObject().GetName()
	}
}

func (a *ObjectSetAdapter) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.ObjectSetTemplateSpec.Phases
}

func (a *ObjectSetAdapter) SetPhases(phases []corev1alpha1.ObjectSetTemplatePhase) {
	a.Spec.Phases = phases
}

func (a *ObjectSetAdapter) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *ObjectSetAdapter) GetRevision() int64 {
	return a.Status.Revision
}

func (a *ObjectSetAdapter) SetRevision(revision int64) {
	a.Status.Revision = revision
}

func (a *ObjectSetAdapter) GetGeneration() int64 {
	return a.Generation
}

func (a *ObjectSetAdapter) IsArchived() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *ObjectSetAdapter) SetArchived() {
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *ObjectSetAdapter) IsStatusPaused() bool {
	return meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetPaused,
	)
}

func (a *ObjectSetAdapter) IsSpecPaused() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ObjectSetAdapter) SetPaused() {
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ObjectSetAdapter) IsAvailable() bool {
	return meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetAvailable,
	)
}

func (a *ObjectSetAdapter) GetPausedByParent() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused &&
		a.Annotations[pausedByParentAnnotation] == pausedByParentTrue
}

func (a *ObjectSetAdapter) SetPausedByParent() {
	if a.Annotations == nil {
		a.Annotations = map[string]string{}
	}
	a.Annotations[pausedByParentAnnotation] = pausedByParentTrue
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ObjectSetAdapter) SetActiveByParent() {
	delete(a.Annotations, pausedByParentAnnotation)
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateActive
}

func (a *ObjectSetAdapter) GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe {
	return a.Spec.AvailabilityProbes
}

func (a *ObjectSetAdapter) GetSuccessDelaySeconds() int32 {
	return a.Spec.SuccessDelaySeconds
}

func (a *ObjectSetAdapter) GetRemotePhases() []corev1alpha1.RemotePhaseReference {
	return a.Status.RemotePhases
}

func (a *ObjectSetAdapter) SetRemotePhases(remotes []corev1alpha1.RemotePhaseReference) {
	a.Status.RemotePhases = remotes
}

func (a *ObjectSetAdapter) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	return a.Status.ControllerOf
}

func (a *ObjectSetAdapter) SetStatusControllerOf(controllerOf []corev1alpha1.ControlledObjectReference) {
	a.Status.ControllerOf = controllerOf
}

type ClusterObjectSetAdapter struct {
	corev1alpha1.ClusterObjectSet
}

func (a *ClusterObjectSetAdapter) ClientObject() client.Object {
	return &a.ClusterObjectSet
}

func (a *ClusterObjectSetAdapter) GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	return a.Spec.ObjectSetTemplateSpec
}

func (a *ClusterObjectSetAdapter) SetTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec) {
	a.Spec.ObjectSetTemplateSpec = templateSpec
}

func (a *ClusterObjectSetAdapter) GetPrevious() []corev1alpha1.PreviousRevisionReference {
	return a.Spec.Previous
}

func (a *ClusterObjectSetAdapter) SetPreviousRevisions(prevObjectSets []ObjectSetAccessor) {
	a.Spec.Previous = make([]corev1alpha1.PreviousRevisionReference, len(prevObjectSets))
	for i := range prevObjectSets {
		a.Spec.Previous[i].Name = prevObjectSets[i].ClientObject().GetName()
	}
}

func (a *ClusterObjectSetAdapter) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.ObjectSetTemplateSpec.Phases
}

func (a *ClusterObjectSetAdapter) SetPhases(phases []corev1alpha1.ObjectSetTemplatePhase) {
	a.Spec.Phases = phases
}

func (a *ClusterObjectSetAdapter) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *ClusterObjectSetAdapter) GetRevision() int64 {
	return a.Status.Revision
}

func (a *ClusterObjectSetAdapter) SetRevision(revision int64) {
	a.Status.Revision = revision
}

func (a *ClusterObjectSetAdapter) GetGeneration() int64 {
	return a.Generation
}

func (a *ClusterObjectSetAdapter) IsArchived() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *ClusterObjectSetAdapter) SetArchived() {
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *ClusterObjectSetAdapter) IsStatusPaused() bool {
	return meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetPaused,
	)
}

func (a *ClusterObjectSetAdapter) IsSpecPaused() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ClusterObjectSetAdapter) SetPaused() {
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ClusterObjectSetAdapter) IsAvailable() bool {
	return meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetAvailable,
	)
}

func (a *ClusterObjectSetAdapter) GetPausedByParent() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused &&
		a.Annotations[pausedByParentAnnotation] == pausedByParentTrue
}

func (a *ClusterObjectSetAdapter) SetPausedByParent() {
	if a.Annotations == nil {
		a.Annotations = map[string]string{}
	}
	a.Annotations[pausedByParentAnnotation] = pausedByParentTrue
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ClusterObjectSetAdapter) SetActiveByParent() {
	delete(a.Annotations, pausedByParentAnnotation)
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateActive
}

func (a *ClusterObjectSetAdapter) GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe {
	return a.Spec.AvailabilityProbes
}

func (a *ClusterObjectSetAdapter) GetSuccessDelaySeconds() int32 {
	return a.Spec.SuccessDelaySeconds
}

func (a *ClusterObjectSetAdapter) GetRemotePhases() []corev1alpha1.RemotePhaseReference {
	return a.Status.RemotePhases
}

func (a *ClusterObjectSetAdapter) SetRemotePhases(remotes []corev1alpha1.RemotePhaseReference) {
	a.Status.RemotePhases = remotes
}

func (a *ClusterObjectSetAdapter) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	return a.Status.ControllerOf
}

func (a *ClusterObjectSetAdapter) SetStatusControllerOf(controllerOf []corev1alpha1.ControlledObjectReference) {
	a.Status.ControllerOf = controllerOf
}
