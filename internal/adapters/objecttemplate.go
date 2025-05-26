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
	GetTemplate() string
	GetSources() []corev1alpha1.ObjectTemplateSource
	GetStatusConditions() *[]metav1.Condition
	GetGeneration() int64
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

func (t *GenericObjectTemplate) GetTemplate() string {
	return t.Spec.Template
}

func (t *GenericObjectTemplate) GetSources() []corev1alpha1.ObjectTemplateSource {
	return t.Spec.Sources
}

func (t *GenericObjectTemplate) GetStatusConditions() *[]metav1.Condition {
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

func (t *GenericClusterObjectTemplate) GetTemplate() string {
	return t.Spec.Template
}

func (t *GenericClusterObjectTemplate) GetSources() []corev1alpha1.ObjectTemplateSource {
	return t.Spec.Sources
}

func (t *GenericClusterObjectTemplate) GetStatusConditions() *[]metav1.Condition {
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
