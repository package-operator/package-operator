package objecttemplate

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type genericObjectTemplate interface {
	ClientObject() client.Object
	GetTemplate() string
	GetSources() []corev1alpha1.ObjectTemplateSource
	GetConditions() *[]metav1.Condition
	GetGeneration() int64
	UpdatePhase()
	SetStatusControllerOf(corev1alpha1.ControlledObjectReference)
	GetStatusControllerOf() corev1alpha1.ControlledObjectReference
}

type genericObjectTemplateFactory func(
	scheme *runtime.Scheme) genericObjectTemplate

var (
	objectTemplateGVK        = corev1alpha1.GroupVersion.WithKind("ObjectTemplate")
	clusterObjectTemplateGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectTemplate")
)

func newGenericObjectTemplate(scheme *runtime.Scheme) genericObjectTemplate {
	obj, err := scheme.New(objectTemplateGVK)
	if err != nil {
		panic(err)
	}

	return &GenericObjectTemplate{
		ObjectTemplate: *obj.(*corev1alpha1.ObjectTemplate),
	}
}

func newGenericClusterObjectTemplate(scheme *runtime.Scheme) genericObjectTemplate {
	obj, err := scheme.New(clusterObjectTemplateGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterObjectTemplate{
		ClusterObjectTemplate: *obj.(*corev1alpha1.ClusterObjectTemplate),
	}
}

type GenericObjectTemplate struct {
	corev1alpha1.ObjectTemplate
}

func (t *GenericObjectTemplate) ClientObject() client.Object {
	return &t.ObjectTemplate
}

func (t *GenericObjectTemplate) GetTemplate() string {
	return t.Spec.Template
}

func (t *GenericObjectTemplate) GetSources() []corev1alpha1.ObjectTemplateSource {
	return t.Spec.Sources
}

func (t *GenericObjectTemplate) GetConditions() *[]metav1.Condition {
	return &t.Status.Conditions
}

func (t *GenericObjectTemplate) GetGeneration() int64 {
	return t.Generation
}

func (t *GenericObjectTemplate) UpdatePhase() {
	t.Status.Phase = getObjectTemplatePhase(t)
}

func (t *GenericObjectTemplate) SetStatusControllerOf(controllerOf corev1alpha1.ControlledObjectReference) {
	t.Status.ControllerOf = controllerOf
}

func (t *GenericObjectTemplate) GetStatusControllerOf() corev1alpha1.ControlledObjectReference {
	return t.Status.ControllerOf
}

type GenericClusterObjectTemplate struct {
	corev1alpha1.ClusterObjectTemplate
}

func (t *GenericClusterObjectTemplate) GetTemplate() string {
	return t.Spec.Template
}

func (t *GenericClusterObjectTemplate) GetSources() []corev1alpha1.ObjectTemplateSource {
	return t.Spec.Sources
}

func (t *GenericClusterObjectTemplate) GetConditions() *[]metav1.Condition {
	return &t.Status.Conditions
}

func (t *GenericClusterObjectTemplate) ClientObject() client.Object {
	return &t.ClusterObjectTemplate
}

func (t *GenericClusterObjectTemplate) UpdatePhase() {
	t.Status.Phase = getObjectTemplatePhase(t)
}

func (t *GenericClusterObjectTemplate) GetGeneration() int64 {
	return t.Generation
}

func getObjectTemplatePhase(objectTemplate genericObjectTemplate) corev1alpha1.ObjectTemplateStatusPhase {
	if meta.IsStatusConditionTrue(*objectTemplate.GetConditions(), corev1alpha1.ObjectTemplateInvalid) {
		return corev1alpha1.ObjectTemplatePhaseError
	}
	return corev1alpha1.ObjectTemplatePhaseActive
}

func (t *GenericClusterObjectTemplate) SetStatusControllerOf(controllerOf corev1alpha1.ControlledObjectReference) {
	t.Status.ControllerOf = controllerOf
}

func (t *GenericClusterObjectTemplate) GetStatusControllerOf() corev1alpha1.ControlledObjectReference {
	return t.Status.ControllerOf
}
