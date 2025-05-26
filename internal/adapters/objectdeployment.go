package adapters

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// ObjectDeploymentAccessor is an adapter interface to access an ObjectDeployment.
//
// Reason for this interface is that it allows accessing an ObjectDeployment in two scopes:
// The regular ObjectDeployment and the ClusterObjectDeployment.
type ObjectDeploymentAccessor interface {
	ClientObject() client.Object
	GetGeneration() int64

	GetSpecObjectSetTemplate() corev1alpha1.ObjectSetTemplate
	GetSpecPaused() bool
	SetSpecPaused(paused bool)
	GetSpecRevisionHistoryLimit() *int32
	GetSpecSelector() metav1.LabelSelector
	SetSpecSelector(labels map[string]string)
	SetSpecTemplateSpec(corev1alpha1.ObjectSetTemplateSpec)
	GetSpecTemplateSpec() corev1alpha1.ObjectSetTemplateSpec

	GetStatusCollisionCount() *int32
	SetStatusCollisionCount(*int32)
	GetStatusConditions() *[]metav1.Condition
	SetStatusConditions(...metav1.Condition)
	RemoveStatusConditions(...string)
	GetStatusControllerOf() []corev1alpha1.ControlledObjectReference
	SetStatusControllerOf([]corev1alpha1.ControlledObjectReference)
	GetStatusRevision() int64
	SetStatusRevision(r int64)
	GetStatusTemplateHash() string
	SetStatusTemplateHash(templateHash string)
}

var (
	_ ObjectDeploymentAccessor = (*ObjectDeployment)(nil)
	_ ObjectDeploymentAccessor = (*ClusterObjectDeployment)(nil)
)

type ObjectDeploymentFactory func(scheme *runtime.Scheme) ObjectDeploymentAccessor

var (
	objectDeploymentGVK        = corev1alpha1.GroupVersion.WithKind("ObjectDeployment")
	clusterObjectDeploymentGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectDeployment")
)

func NewObjectDeployment(scheme *runtime.Scheme) ObjectDeploymentAccessor {
	obj, err := scheme.New(objectDeploymentGVK)
	if err != nil {
		panic(err)
	}

	return &ObjectDeployment{ObjectDeployment: *obj.(*corev1alpha1.ObjectDeployment)}
}

func NewClusterObjectDeployment(scheme *runtime.Scheme) ObjectDeploymentAccessor {
	obj, err := scheme.New(clusterObjectDeploymentGVK)
	if err != nil {
		panic(err)
	}

	return &ClusterObjectDeployment{ClusterObjectDeployment: *obj.(*corev1alpha1.ClusterObjectDeployment)}
}

var (
	_ ObjectDeploymentAccessor = (*ObjectDeployment)(nil)
	_ ObjectDeploymentAccessor = (*ClusterObjectDeployment)(nil)
)

type ObjectDeployment struct {
	corev1alpha1.ObjectDeployment
}

func (a *ObjectDeployment) GetSpecRevisionHistoryLimit() *int32 {
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

func (a *ObjectDeployment) GetStatusConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *ObjectDeployment) GetSpecSelector() metav1.LabelSelector {
	return a.Spec.Selector
}

func (a *ObjectDeployment) GetSpecObjectSetTemplate() corev1alpha1.ObjectSetTemplate {
	return a.Spec.Template
}

func (a *ObjectDeployment) SetStatusConditions(conds ...metav1.Condition) {
	for _, c := range conds {
		c.ObservedGeneration = a.ClientObject().GetGeneration()

		meta.SetStatusCondition(&a.Status.Conditions, c)
	}
}

func (a *ObjectDeployment) RemoveStatusConditions(condTypes ...string) {
	for _, ct := range condTypes {
		meta.RemoveStatusCondition(&a.Status.Conditions, ct)
	}
}

func (a *ObjectDeployment) SetStatusTemplateHash(templateHash string) {
	a.Status.TemplateHash = templateHash
}

func (a *ObjectDeployment) GetStatusTemplateHash() string {
	return a.Status.TemplateHash
}

func (a *ObjectDeployment) SetSpecTemplateSpec(spec corev1alpha1.ObjectSetTemplateSpec) {
	a.Spec.Template.Spec = spec
}

func (a *ObjectDeployment) GetSpecTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	return a.Spec.Template.Spec
}

func (a *ObjectDeployment) SetSpecSelector(labels map[string]string) {
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

func (a *ObjectDeployment) GetSpecPaused() bool {
	return a.Spec.Paused
}

func (a *ObjectDeployment) SetSpecPaused(paused bool) {
	a.Spec.Paused = paused
}

type ClusterObjectDeployment struct {
	corev1alpha1.ClusterObjectDeployment
}

func (a *ClusterObjectDeployment) GetSpecRevisionHistoryLimit() *int32 {
	return a.Spec.RevisionHistoryLimit
}

func (a *ClusterObjectDeployment) SetStatusCollisionCount(cc *int32) {
	a.Status.CollisionCount = cc
}

func (a *ClusterObjectDeployment) ClientObject() client.Object {
	return &a.ClusterObjectDeployment
}

func (a *ClusterObjectDeployment) GetStatusConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *ClusterObjectDeployment) GetSpecSelector() metav1.LabelSelector {
	return a.Spec.Selector
}

func (a *ClusterObjectDeployment) GetSpecObjectSetTemplate() corev1alpha1.ObjectSetTemplate {
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

func (a *ClusterObjectDeployment) RemoveStatusConditions(condTypes ...string) {
	for _, ct := range condTypes {
		meta.RemoveStatusCondition(&a.Status.Conditions, ct)
	}
}

func (a *ClusterObjectDeployment) SetStatusTemplateHash(templateHash string) {
	a.Status.TemplateHash = templateHash
}

func (a *ClusterObjectDeployment) GetStatusTemplateHash() string {
	return a.Status.TemplateHash
}

func (a *ClusterObjectDeployment) SetSpecTemplateSpec(spec corev1alpha1.ObjectSetTemplateSpec) {
	a.Spec.Template.Spec = spec
}

func (a *ClusterObjectDeployment) GetSpecTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	return a.Spec.Template.Spec
}

func (a *ClusterObjectDeployment) SetSpecSelector(labels map[string]string) {
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

func (a *ClusterObjectDeployment) GetSpecPaused() bool {
	return a.Spec.Paused
}

func (a *ClusterObjectDeployment) SetSpecPaused(paused bool) {
	a.Spec.Paused = paused
}
