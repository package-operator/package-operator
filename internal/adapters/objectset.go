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
	ClientObject() client.Object
	GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec
	SetTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec)
	SetPreviousRevisions(prev []ObjectSetAccessor)
	GetPhases() []corev1alpha1.ObjectSetTemplatePhase
	GetConditions() *[]metav1.Condition
	SetArchived()
	IsArchived() bool
	GetRevision() int64
	GetGeneration() int64
	IsStatusPaused() bool
	SetPaused()
	IsSpecPaused() bool
	IsAvailable() bool
	SetPausedByParent()
	SetActiveByParent()
	GetPausedByParent() bool
	IsPaused() bool
	GetPrevious() []corev1alpha1.PreviousRevisionReference
	SetPhases(phases []corev1alpha1.ObjectSetTemplatePhase)
	GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe
	GetSuccessDelaySeconds() int32
	SetRevision(revision int64)
	GetRemotePhases() []corev1alpha1.RemotePhaseReference
	SetRemotePhases([]corev1alpha1.RemotePhaseReference)
	GetStatusControllerOf() []corev1alpha1.ControlledObjectReference
	SetStatusControllerOf([]corev1alpha1.ControlledObjectReference)
}

type ObjectSetAccessorFactory func(
	scheme *runtime.Scheme) ObjectSetAccessor

var (
	objectSetGVK        = corev1alpha1.GroupVersion.WithKind("ObjectSet")
	clusterObjectSetGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectSet")
)

func NewObjectSet(scheme *runtime.Scheme) ObjectSetAccessor {
	obj, err := scheme.New(objectSetGVK)
	if err != nil {
		panic(err)
	}

	return &ObjectSet{
		ObjectSet: *obj.(*corev1alpha1.ObjectSet),
	}
}

func NewClusterObjectSet(scheme *runtime.Scheme) ObjectSetAccessor {
	obj, err := scheme.New(clusterObjectSetGVK)
	if err != nil {
		panic(err)
	}

	return &ClusterObjectSet{
		ClusterObjectSet: *obj.(*corev1alpha1.ClusterObjectSet),
	}
}

var (
	_ ObjectSetAccessor = (*ObjectSet)(nil)
	_ ObjectSetAccessor = (*ClusterObjectSet)(nil)
)

type ObjectSet struct {
	corev1alpha1.ObjectSet
}

func (a *ObjectSet) GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	return a.Spec.ObjectSetTemplateSpec
}

func (a *ObjectSet) IsStatusPaused() bool {
	return meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetPaused,
	)
}

func (a *ObjectSet) IsAvailable() bool {
	return meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetAvailable,
	)
}

func (a *ObjectSet) SetPaused() {
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ObjectSet) IsSpecPaused() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ObjectSet) SetTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec) {
	a.Spec.ObjectSetTemplateSpec = templateSpec
}

func (a *ObjectSet) ClientObject() client.Object {
	return &a.ObjectSet
}

func (a *ObjectSet) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *ObjectSet) GetRevision() int64 {
	return a.Status.Revision
}

func (a *ObjectSet) GetGeneration() int64 {
	return a.Generation
}

func (a *ObjectSet) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.ObjectSetTemplateSpec.Phases
}

func (a *ObjectSet) SetArchived() {
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *ObjectSet) IsArchived() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *ObjectSet) SetPreviousRevisions(prevObjectSets []ObjectSetAccessor) {
	prevRefs := make([]corev1alpha1.PreviousRevisionReference, len(prevObjectSets))
	for i := range prevObjectSets {
		prevObjSet := prevObjectSets[i]
		currPrevRef := corev1alpha1.PreviousRevisionReference{
			Name: prevObjSet.ClientObject().GetName(),
		}
		prevRefs[i] = currPrevRef
	}
	a.Spec.Previous = prevRefs
}

func (a *ObjectSet) SetPausedByParent() {
	a.Annotations[pausedByParentAnnotation] = pausedByParentTrue
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ObjectSet) SetActiveByParent() {
	delete(a.Annotations, pausedByParentAnnotation)
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateActive
}

func (a *ObjectSet) GetPausedByParent() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused &&
		a.Annotations[pausedByParentAnnotation] == pausedByParentTrue
}

func (a *ObjectSet) IsPaused() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ObjectSet) GetPrevious() []corev1alpha1.PreviousRevisionReference {
	return a.Spec.Previous
}

func (a *ObjectSet) SetPhases(phases []corev1alpha1.ObjectSetTemplatePhase) {
	a.Spec.Phases = phases
}

func (a *ObjectSet) GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe {
	return a.Spec.AvailabilityProbes
}

func (a *ObjectSet) GetSuccessDelaySeconds() int32 {
	return a.Spec.SuccessDelaySeconds
}

func (a *ObjectSet) SetRevision(revision int64) {
	a.Status.Revision = revision
}

func (a *ObjectSet) GetRemotePhases() []corev1alpha1.RemotePhaseReference {
	return a.Status.RemotePhases
}

func (a *ObjectSet) SetRemotePhases(remotes []corev1alpha1.RemotePhaseReference) {
	a.Status.RemotePhases = remotes
}

func (a *ObjectSet) SetStatusControllerOf(controllerOf []corev1alpha1.ControlledObjectReference) {
	a.Status.ControllerOf = controllerOf
}

func (a *ObjectSet) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	return a.Status.ControllerOf
}

type ClusterObjectSet struct {
	corev1alpha1.ClusterObjectSet
}

func (a *ClusterObjectSet) SetArchived() {
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *ClusterObjectSet) SetPreviousRevisions(prevObjectSets []ObjectSetAccessor) {
	prevRefs := make([]corev1alpha1.PreviousRevisionReference, len(prevObjectSets))
	for i := range prevObjectSets {
		prevObjSet := prevObjectSets[i]
		currPrevRef := corev1alpha1.PreviousRevisionReference{
			Name: prevObjSet.ClientObject().GetName(),
		}
		prevRefs[i] = currPrevRef
	}
	a.Spec.Previous = prevRefs
}

func (a *ClusterObjectSet) GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	return a.Spec.ObjectSetTemplateSpec
}

func (a *ClusterObjectSet) SetTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec) {
	a.Spec.ObjectSetTemplateSpec = templateSpec
}

func (a *ClusterObjectSet) ClientObject() client.Object {
	return &a.ClusterObjectSet
}

func (a *ClusterObjectSet) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *ClusterObjectSet) IsAvailable() bool {
	return meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetAvailable,
	)
}

func (a *ClusterObjectSet) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.Phases
}

func (a *ClusterObjectSet) GetRevision() int64 {
	return a.Status.Revision
}

func (a *ClusterObjectSet) GetGeneration() int64 {
	return a.Generation
}

func (a *ClusterObjectSet) IsArchived() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *ClusterObjectSet) IsStatusPaused() bool {
	return meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetPaused,
	)
}

func (a *ClusterObjectSet) SetPaused() {
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ClusterObjectSet) IsSpecPaused() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ClusterObjectSet) SetPausedByParent() {
	a.Annotations[pausedByParentAnnotation] = pausedByParentTrue
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ClusterObjectSet) SetActiveByParent() {
	delete(a.Annotations, pausedByParentAnnotation)
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateActive
}

func (a *ClusterObjectSet) GetPausedByParent() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused &&
		a.Annotations[pausedByParentAnnotation] == pausedByParentTrue
}

func (a *ClusterObjectSet) IsPaused() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ClusterObjectSet) GetPrevious() []corev1alpha1.PreviousRevisionReference {
	return a.Spec.Previous
}

func (a *ClusterObjectSet) SetPhases(phases []corev1alpha1.ObjectSetTemplatePhase) {
	a.Spec.Phases = phases
}

func (a *ClusterObjectSet) GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe {
	return a.Spec.AvailabilityProbes
}

func (a *ClusterObjectSet) GetSuccessDelaySeconds() int32 {
	return a.Spec.SuccessDelaySeconds
}

func (a *ClusterObjectSet) SetRevision(revision int64) {
	a.Status.Revision = revision
}

func (a *ClusterObjectSet) GetRemotePhases() []corev1alpha1.RemotePhaseReference {
	return a.Status.RemotePhases
}

func (a *ClusterObjectSet) SetRemotePhases(remotes []corev1alpha1.RemotePhaseReference) {
	a.Status.RemotePhases = remotes
}

func (a *ClusterObjectSet) SetStatusControllerOf(controllerOf []corev1alpha1.ControlledObjectReference) {
	a.Status.ControllerOf = controllerOf
}

func (a *ClusterObjectSet) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	return a.Status.ControllerOf
}
