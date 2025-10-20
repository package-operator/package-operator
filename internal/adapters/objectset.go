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

var (
	_ ObjectSetAccessor = (*ObjectSetAdapter)(nil)
	_ ObjectSetAccessor = (*ClusterObjectSetAdapter)(nil)
)

// ObjectSetAccessor is an adapter interface to access an ObjectSet.
//
// Reason for this interface is that it allows accessing an ObjectSet in two scopes:
// The regular ObjectSet and the ClusterObjectSet.
type ObjectSetAccessor interface {
	ClientObject() client.Object
	GetGeneration() int64

	IsSpecArchived() bool
	SetSpecActiveByParent()
	IsSpecAvailable() bool
	SetSpecArchived()
	GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe
	GetSpecTemplateSpec() corev1alpha1.ObjectSetTemplateSpec
	SetSpecTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec)
	IsSpecPaused() bool
	SetSpecPaused()
	GetSpecPausedByParent() bool
	SetSpecPausedByParent()
	GetSpecPhases() []corev1alpha1.ObjectSetTemplatePhase
	SetSpecPhases(phases []corev1alpha1.ObjectSetTemplatePhase)
	GetSpecPrevious() []corev1alpha1.PreviousRevisionReference
	SetSpecPreviousRevisions(prev []ObjectSetAccessor)
	GetSpecSuccessDelaySeconds() int32
	SetSpecRevision(int64)
	GetSpecRevision() int64

	IsStatusPaused() bool
	// Deprecated: use GetSpecRevision instead
	GetStatusRevision() int64
	// Deprecated: use SetSpecRevision instead
	SetStatusRevision(revision int64)
	GetStatusConditions() *[]metav1.Condition
	GetStatusRemotePhases() []corev1alpha1.RemotePhaseReference
	SetStatusRemotePhases([]corev1alpha1.RemotePhaseReference)
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

	return &ObjectSetAdapter{ObjectSet: *obj.(*corev1alpha1.ObjectSet)}
}

func NewClusterObjectSet(scheme *runtime.Scheme) ObjectSetAccessor {
	obj, err := scheme.New(clusterObjectSetGVK)
	if err != nil {
		panic(err)
	}

	return &ClusterObjectSetAdapter{ClusterObjectSet: *obj.(*corev1alpha1.ClusterObjectSet)}
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

func (a *ObjectSetAdapter) GetSpecTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	return a.Spec.ObjectSetTemplateSpec
}

func (a *ObjectSetAdapter) SetSpecTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec) {
	a.Spec.ObjectSetTemplateSpec = templateSpec
}

func (a *ObjectSetAdapter) GetSpecPrevious() []corev1alpha1.PreviousRevisionReference {
	return a.Spec.Previous
}

func (a *ObjectSetAdapter) SetSpecPreviousRevisions(prevObjectSets []ObjectSetAccessor) {
	a.Spec.Previous = make([]corev1alpha1.PreviousRevisionReference, len(prevObjectSets))
	for i := range prevObjectSets {
		a.Spec.Previous[i].Name = prevObjectSets[i].ClientObject().GetName()
	}
}

func (a *ObjectSetAdapter) GetSpecPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.Phases
}

func (a *ObjectSetAdapter) SetSpecPhases(phases []corev1alpha1.ObjectSetTemplatePhase) {
	a.Spec.Phases = phases
}

func (a *ObjectSetAdapter) GetStatusConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *ObjectSetAdapter) GetStatusRevision() int64 {
	return a.Status.Revision //nolint:staticcheck
}

func (a *ObjectSetAdapter) SetStatusRevision(revision int64) {
	a.Status.Revision = revision //nolint:staticcheck
}

func (a *ObjectSetAdapter) GetGeneration() int64 {
	return a.Generation
}

func (a *ObjectSetAdapter) IsSpecArchived() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *ObjectSetAdapter) SetSpecArchived() {
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

func (a *ObjectSetAdapter) SetSpecPaused() {
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ObjectSetAdapter) IsSpecAvailable() bool {
	return meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetAvailable,
	)
}

func (a *ObjectSetAdapter) GetSpecPausedByParent() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused &&
		a.Annotations[pausedByParentAnnotation] == pausedByParentTrue
}

func (a *ObjectSetAdapter) SetSpecPausedByParent() {
	if a.Annotations == nil {
		a.Annotations = map[string]string{}
	}
	a.Annotations[pausedByParentAnnotation] = pausedByParentTrue
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ObjectSetAdapter) SetSpecActiveByParent() {
	delete(a.Annotations, pausedByParentAnnotation)
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateActive
}

func (a *ObjectSetAdapter) GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe {
	return a.Spec.AvailabilityProbes
}

func (a *ObjectSetAdapter) GetSpecSuccessDelaySeconds() int32 {
	return a.Spec.SuccessDelaySeconds
}

func (a *ObjectSetAdapter) SetSpecRevision(revision int64) {
	a.Spec.Revision = revision
}

func (a *ObjectSetAdapter) GetSpecRevision() int64 {
	return a.Spec.Revision
}

func (a *ObjectSetAdapter) GetStatusRemotePhases() []corev1alpha1.RemotePhaseReference {
	return a.Status.RemotePhases
}

func (a *ObjectSetAdapter) SetStatusRemotePhases(remotes []corev1alpha1.RemotePhaseReference) {
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

func (a *ClusterObjectSetAdapter) GetSpecTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	return a.Spec.ObjectSetTemplateSpec
}

func (a *ClusterObjectSetAdapter) SetSpecTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec) {
	a.Spec.ObjectSetTemplateSpec = templateSpec
}

func (a *ClusterObjectSetAdapter) GetSpecPrevious() []corev1alpha1.PreviousRevisionReference {
	return a.Spec.Previous
}

func (a *ClusterObjectSetAdapter) SetSpecPreviousRevisions(prevObjectSets []ObjectSetAccessor) {
	a.Spec.Previous = make([]corev1alpha1.PreviousRevisionReference, len(prevObjectSets))
	for i := range prevObjectSets {
		a.Spec.Previous[i].Name = prevObjectSets[i].ClientObject().GetName()
	}
}

func (a *ClusterObjectSetAdapter) GetSpecPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.Phases
}

func (a *ClusterObjectSetAdapter) SetSpecPhases(phases []corev1alpha1.ObjectSetTemplatePhase) {
	a.Spec.Phases = phases
}

func (a *ClusterObjectSetAdapter) GetStatusConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *ClusterObjectSetAdapter) GetStatusRevision() int64 {
	return a.Status.Revision //nolint:staticcheck
}

func (a *ClusterObjectSetAdapter) SetStatusRevision(revision int64) {
	a.Status.Revision = revision //nolint:staticcheck
}

func (a *ClusterObjectSetAdapter) GetGeneration() int64 {
	return a.Generation
}

func (a *ClusterObjectSetAdapter) IsSpecArchived() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *ClusterObjectSetAdapter) SetSpecArchived() {
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

func (a *ClusterObjectSetAdapter) SetSpecPaused() {
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ClusterObjectSetAdapter) IsSpecAvailable() bool {
	return meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetAvailable,
	)
}

func (a *ClusterObjectSetAdapter) GetSpecPausedByParent() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused &&
		a.Annotations[pausedByParentAnnotation] == pausedByParentTrue
}

func (a *ClusterObjectSetAdapter) SetSpecPausedByParent() {
	if a.Annotations == nil {
		a.Annotations = map[string]string{}
	}
	a.Annotations[pausedByParentAnnotation] = pausedByParentTrue
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *ClusterObjectSetAdapter) SetSpecActiveByParent() {
	delete(a.Annotations, pausedByParentAnnotation)
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateActive
}

func (a *ClusterObjectSetAdapter) GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe {
	return a.Spec.AvailabilityProbes
}

func (a *ClusterObjectSetAdapter) GetSpecSuccessDelaySeconds() int32 {
	return a.Spec.SuccessDelaySeconds
}

func (a *ClusterObjectSetAdapter) SetSpecRevision(revision int64) {
	a.Spec.Revision = revision
}

func (a *ClusterObjectSetAdapter) GetSpecRevision() int64 {
	return a.Spec.Revision
}

func (a *ClusterObjectSetAdapter) GetStatusRemotePhases() []corev1alpha1.RemotePhaseReference {
	return a.Status.RemotePhases
}

func (a *ClusterObjectSetAdapter) SetStatusRemotePhases(remotes []corev1alpha1.RemotePhaseReference) {
	a.Status.RemotePhases = remotes
}

func (a *ClusterObjectSetAdapter) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	return a.Status.ControllerOf
}

func (a *ClusterObjectSetAdapter) SetStatusControllerOf(controllerOf []corev1alpha1.ControlledObjectReference) {
	a.Status.ControllerOf = controllerOf
}
