package objectsets

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type genericObjectSet interface {
	ClientObject() client.Object
	UpdateStatusPhase()
	GetConditions() *[]metav1.Condition
	IsArchived() bool
	IsPaused() bool
	GetPrevious() []corev1alpha1.PreviousRevisionReference
	GetPhases() []corev1alpha1.ObjectSetTemplatePhase
	GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe
	SetRevision(revision int64)
	GetRevision() int64
	GetRemotePhases() []corev1alpha1.RemotePhaseReference
	SetRemotePhases([]corev1alpha1.RemotePhaseReference)
}

type genericObjectSetFactory func(
	scheme *runtime.Scheme) genericObjectSet

var (
	objectSetGVK        = corev1alpha1.GroupVersion.WithKind("ObjectSet")
	clusterObjectSetGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectSet")
)

func newGenericObjectSet(scheme *runtime.Scheme) genericObjectSet {
	obj, err := scheme.New(objectSetGVK)
	if err != nil {
		panic(err)
	}

	return &GenericObjectSet{
		ObjectSet: *obj.(*corev1alpha1.ObjectSet)}
}

func newGenericClusterObjectSet(scheme *runtime.Scheme) genericObjectSet {
	obj, err := scheme.New(clusterObjectSetGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterObjectSet{
		ClusterObjectSet: *obj.(*corev1alpha1.ClusterObjectSet)}
}

var (
	_ genericObjectSet = (*GenericObjectSet)(nil)
	_ genericObjectSet = (*GenericClusterObjectSet)(nil)
)

type GenericObjectSet struct {
	corev1alpha1.ObjectSet
}

func (a *GenericObjectSet) ClientObject() client.Object {
	return &a.ObjectSet
}

func (a *GenericObjectSet) UpdateStatusPhase() {
	if meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetArchived,
	) {
		a.Status.Phase = corev1alpha1.ObjectSetStatusPhaseArchived
		return
	}

	if meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetPaused,
	) {
		a.Status.Phase = corev1alpha1.ObjectSetStatusPhasePaused
		return
	}

	availableCond := meta.FindStatusCondition(
		a.Status.Conditions,
		corev1alpha1.ObjectSetAvailable,
	)
	if availableCond != nil {
		if availableCond.Status == metav1.ConditionTrue {
			a.Status.Phase = corev1alpha1.ObjectSetStatusPhaseAvailable
			return
		}
	}

	a.Status.Phase = corev1alpha1.ObjectSetStatusPhaseNotReady
}

func (a *GenericObjectSet) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericObjectSet) IsPaused() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *GenericObjectSet) IsArchived() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *GenericObjectSet) GetPrevious() []corev1alpha1.PreviousRevisionReference {
	return a.Spec.Previous
}

func (a *GenericObjectSet) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.Phases
}

func (a *GenericObjectSet) GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe {
	return a.Spec.AvailabilityProbes
}

func (a *GenericObjectSet) SetRevision(revision int64) {
	a.Status.Revision = revision
}

func (a *GenericObjectSet) GetRevision() int64 {
	return a.Status.Revision
}

func (a *GenericObjectSet) GetRemotePhases() []corev1alpha1.RemotePhaseReference {
	return a.Status.RemotePhases
}

func (a *GenericObjectSet) SetRemotePhases(remotes []corev1alpha1.RemotePhaseReference) {
	a.Status.RemotePhases = remotes
}

type GenericClusterObjectSet struct {
	corev1alpha1.ClusterObjectSet
}

func (a *GenericClusterObjectSet) ClientObject() client.Object {
	return &a.ClusterObjectSet
}

func (a *GenericClusterObjectSet) UpdateStatusPhase() {
	if meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetArchived,
	) {
		a.Status.Phase = corev1alpha1.ObjectSetStatusPhaseArchived
		return
	}

	if meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetPaused,
	) {
		a.Status.Phase = corev1alpha1.ObjectSetStatusPhasePaused
		return
	}

	availableCond := meta.FindStatusCondition(
		a.Status.Conditions,
		corev1alpha1.ObjectSetAvailable,
	)
	if availableCond != nil {
		if availableCond.Status == metav1.ConditionTrue {
			a.Status.Phase = corev1alpha1.ObjectSetStatusPhaseAvailable
			return
		}
	}

	a.Status.Phase = corev1alpha1.ObjectSetStatusPhaseNotReady
}

func (a *GenericClusterObjectSet) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericClusterObjectSet) IsPaused() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *GenericClusterObjectSet) IsArchived() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *GenericClusterObjectSet) GetPrevious() []corev1alpha1.PreviousRevisionReference {
	return a.Spec.Previous
}

func (a *GenericClusterObjectSet) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.Phases
}

func (a *GenericClusterObjectSet) GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe {
	return a.Spec.AvailabilityProbes
}

func (a *GenericClusterObjectSet) SetRevision(revision int64) {
	a.Status.Revision = revision
}

func (a *GenericClusterObjectSet) GetRevision() int64 {
	return a.Status.Revision
}

func (a *GenericClusterObjectSet) GetRemotePhases() []corev1alpha1.RemotePhaseReference {
	return a.Status.RemotePhases
}

func (a *GenericClusterObjectSet) SetRemotePhases(remotes []corev1alpha1.RemotePhaseReference) {
	a.Status.RemotePhases = remotes
}
