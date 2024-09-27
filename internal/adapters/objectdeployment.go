package adapters

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type ObjectDeploymentAccessor interface {
	ClientObject() client.Object
	UpdatePhase()
	GetConditions() *[]metav1.Condition
	GetSelector() metav1.LabelSelector
	GetObjectSetTemplate() corev1alpha1.ObjectSetTemplate
	SetTemplateSpec(corev1alpha1.ObjectSetTemplateSpec)
	GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec
	GetRevisionHistoryLimit() *int32
	SetStatusConditions(...metav1.Condition)
	SetStatusCollisionCount(*int32)
	GetStatusCollisionCount() *int32
	GetGeneration() int64
	GetStatusTemplateHash() string
	SetStatusTemplateHash(templateHash string)
	SetSelector(labels map[string]string)
	SetStatusRevision(r int64)
	GetStatusRevision() int64
	SetStatusControllerOf([]corev1alpha1.ControlledObjectReference)
	GetStatusControllerOf() []corev1alpha1.ControlledObjectReference
}

type ObjectDeploymentFactory func(
	scheme *runtime.Scheme) ObjectDeploymentAccessor

var (
	objectDeploymentGVK        = corev1alpha1.GroupVersion.WithKind("ObjectDeployment")
	clusterObjectDeploymentGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectDeployment")
)

func NewObjectDeployment(scheme *runtime.Scheme) ObjectDeploymentAccessor {
	obj, err := scheme.New(objectDeploymentGVK)
	if err != nil {
		panic(err)
	}

	return &ObjectDeployment{
		ObjectDeployment: *obj.(*corev1alpha1.ObjectDeployment),
	}
}

func NewClusterObjectDeployment(scheme *runtime.Scheme) ObjectDeploymentAccessor {
	obj, err := scheme.New(clusterObjectDeploymentGVK)
	if err != nil {
		panic(err)
	}

	return &ClusterObjectDeployment{
		ClusterObjectDeployment: *obj.(*corev1alpha1.ClusterObjectDeployment),
	}
}

var (
	_ ObjectDeploymentAccessor = (*ObjectDeployment)(nil)
	_ ObjectDeploymentAccessor = (*ClusterObjectDeployment)(nil)
)

type ObjectDeployment struct {
	corev1alpha1.ObjectDeployment
}

func (a *ObjectDeployment) GetRevisionHistoryLimit() *int32 {
	return a.Spec.RevisionHistoryLimit
}

func (a *ObjectDeployment) SetStatusCollisionCount(cc *int32) {
	a.Status.CollisionCount = cc
}

func (a *ObjectDeployment) GetStatusCollisionCount() *int32 {
	return a.Status.CollisionCount
}

func (a *ObjectDeployment) ClientObject() client.Object {
	return &a.ObjectDeployment
}

func (a *ObjectDeployment) UpdatePhase() {
	a.Status.Phase = objectDeploymentPhase(a.Status.Conditions)
}

func (a *ObjectDeployment) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *ObjectDeployment) GetSelector() metav1.LabelSelector {
	return a.Spec.Selector
}

func (a *ObjectDeployment) GetObjectSetTemplate() corev1alpha1.ObjectSetTemplate {
	return a.Spec.Template
}

func (a *ObjectDeployment) SetStatusConditions(conds ...metav1.Condition) {
	for _, c := range conds {
		c.ObservedGeneration = a.ClientObject().GetGeneration()

		meta.SetStatusCondition(&a.Status.Conditions, c)
	}
}

func (a *ObjectDeployment) SetStatusTemplateHash(templateHash string) {
	a.Status.TemplateHash = templateHash
}

func (a *ObjectDeployment) GetStatusTemplateHash() string {
	return a.Status.TemplateHash
}

func (a *ObjectDeployment) SetTemplateSpec(spec corev1alpha1.ObjectSetTemplateSpec) {
	a.Spec.Template.Spec = spec
}

func (a *ObjectDeployment) GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	return a.Spec.Template.Spec
}

func (a *ObjectDeployment) SetSelector(labels map[string]string) {
	a.Spec.Selector = metav1.LabelSelector{
		MatchLabels: labels,
	}
	a.Spec.Template.Metadata.Labels = labels
}

func (a *ObjectDeployment) SetStatusRevision(r int64) {
	a.Status.Revision = r
}

func (a *ObjectDeployment) GetStatusRevision() int64 {
	return a.Status.Revision
}

func (a *ObjectDeployment) SetStatusControllerOf(controllerOf []corev1alpha1.ControlledObjectReference) {
	a.Status.ControllerOf = controllerOf
}

func (a *ObjectDeployment) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	return a.Status.ControllerOf
}

type ClusterObjectDeployment struct {
	corev1alpha1.ClusterObjectDeployment
}

func (a *ClusterObjectDeployment) GetRevisionHistoryLimit() *int32 {
	return a.Spec.RevisionHistoryLimit
}

func (a *ClusterObjectDeployment) SetStatusCollisionCount(cc *int32) {
	a.Status.CollisionCount = cc
}

func (a *ClusterObjectDeployment) ClientObject() client.Object {
	return &a.ClusterObjectDeployment
}

func (a *ClusterObjectDeployment) UpdatePhase() {
	a.Status.Phase = objectDeploymentPhase(a.Status.Conditions)
}

func (a *ClusterObjectDeployment) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *ClusterObjectDeployment) GetSelector() metav1.LabelSelector {
	return a.Spec.Selector
}

func (a *ClusterObjectDeployment) GetObjectSetTemplate() corev1alpha1.ObjectSetTemplate {
	return a.Spec.Template
}

func (a *ClusterObjectDeployment) GetStatusCollisionCount() *int32 {
	return a.Status.CollisionCount
}

func (a *ClusterObjectDeployment) SetStatusConditions(conds ...metav1.Condition) {
	for _, c := range conds {
		c.ObservedGeneration = a.ClientObject().GetGeneration()

		meta.SetStatusCondition(&a.Status.Conditions, c)
	}
}

func (a *ClusterObjectDeployment) SetStatusTemplateHash(templateHash string) {
	a.Status.TemplateHash = templateHash
}

func (a *ClusterObjectDeployment) GetStatusTemplateHash() string {
	return a.Status.TemplateHash
}

func (a *ClusterObjectDeployment) SetTemplateSpec(spec corev1alpha1.ObjectSetTemplateSpec) {
	a.Spec.Template.Spec = spec
}

func (a *ClusterObjectDeployment) GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	return a.Spec.Template.Spec
}

func (a *ClusterObjectDeployment) SetSelector(labels map[string]string) {
	a.Spec.Selector = metav1.LabelSelector{
		MatchLabels: labels,
	}
	a.Spec.Template.Metadata.Labels = labels
}

func (a *ClusterObjectDeployment) SetStatusRevision(r int64) {
	a.Status.Revision = r
}

func (a *ClusterObjectDeployment) GetStatusRevision() int64 {
	return a.Status.Revision
}

func (a *ClusterObjectDeployment) SetStatusControllerOf(controllerOf []corev1alpha1.ControlledObjectReference) {
	a.Status.ControllerOf = controllerOf
}

func (a *ClusterObjectDeployment) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	return a.Status.ControllerOf
}

func objectDeploymentPhase(conditions []metav1.Condition) corev1alpha1.ObjectDeploymentPhase {
	availableCond := meta.FindStatusCondition(conditions, corev1alpha1.ObjectDeploymentAvailable)

	if availableCond != nil {
		if availableCond.Status == metav1.ConditionTrue {
			return corev1alpha1.ObjectDeploymentPhaseAvailable
		}
		if availableCond.Status == metav1.ConditionFalse {
			return corev1alpha1.ObjectDeploymentPhaseNotReady
		}
	}
	return corev1alpha1.ObjectDeploymentPhaseProgressing
}
