package adapters

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

var (
	objectTemplateGVK        = corev1alpha1.GroupVersion.WithKind("ObjectTemplate")
	clusterObjectTemplateGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectTemplate")
)

// ObjectTemplateAccessor is an adapter interface to access an ObjectTemplate.
//
// Reason for this interface is that it allows accessing an ObjectTemplate in two scopes:
// The regular ObjectTemplate and the ClusterObjectTemplate.
type ObjectTemplateAccessor interface {
	ClientObject() client.Object
	GetGeneration() int64

	GetSpecConditions() *[]metav1.Condition
	GetSpecSources() []corev1alpha1.ObjectTemplateSource
	GetSpecTemplate() string

	SetStatusControllerOf(corev1alpha1.ControlledObjectReference)
	GetStatusControllerOf() corev1alpha1.ControlledObjectReference
}

type GenericObjectTemplateFactory func(scheme *runtime.Scheme) ObjectTemplateAccessor

func NewGenericObjectTemplate(scheme *runtime.Scheme) ObjectTemplateAccessor {
	obj, err := scheme.New(objectTemplateGVK)
	if err != nil {
		panic(err)
	}

	return &GenericObjectTemplate{
		ObjectTemplate: *obj.(*corev1alpha1.ObjectTemplate),
	}
}

func NewGenericClusterObjectTemplate(scheme *runtime.Scheme) ObjectTemplateAccessor {
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

func (t *GenericObjectTemplate) GetSpecTemplate() string {
	return t.Spec.Template
}

func (t *GenericObjectTemplate) GetSpecSources() []corev1alpha1.ObjectTemplateSource {
	return t.Spec.Sources
}

func (t *GenericObjectTemplate) GetSpecConditions() *[]metav1.Condition {
	return &t.Status.Conditions
}

func (t *GenericObjectTemplate) GetGeneration() int64 {
	return t.Generation
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

func (t *GenericClusterObjectTemplate) GetSpecTemplate() string {
	return t.Spec.Template
}

func (t *GenericClusterObjectTemplate) GetSpecSources() []corev1alpha1.ObjectTemplateSource {
	return t.Spec.Sources
}

func (t *GenericClusterObjectTemplate) GetSpecConditions() *[]metav1.Condition {
	return &t.Status.Conditions
}

func (t *GenericClusterObjectTemplate) ClientObject() client.Object {
	return &t.ClusterObjectTemplate
}

func (t *GenericClusterObjectTemplate) GetGeneration() int64 {
	return t.Generation
}

func (t *GenericClusterObjectTemplate) SetStatusControllerOf(controllerOf corev1alpha1.ControlledObjectReference) {
	t.Status.ControllerOf = controllerOf
}

func (t *GenericClusterObjectTemplate) GetStatusControllerOf() corev1alpha1.ControlledObjectReference {
	return t.Status.ControllerOf
}
