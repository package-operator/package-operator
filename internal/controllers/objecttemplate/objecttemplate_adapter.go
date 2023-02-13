package objecttemplate

import (
	"k8s.io/apimachinery/pkg/runtime"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type genericObjectTemplate interface {
	ClientObject() client.Object
	GetTemplate() string
	GetSources() []corev1alpha1.ObjectTemplateSource
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

type GenericClusterObjectTemplate struct {
	corev1alpha1.ClusterObjectTemplate
}

func (t *GenericClusterObjectTemplate) GetTemplate() string {
	return t.Spec.Template
}

func (t *GenericClusterObjectTemplate) GetSources() []corev1alpha1.ObjectTemplateSource {
	return t.Spec.Sources
}

func (t *GenericClusterObjectTemplate) ClientObject() client.Object {
	return &t.ClusterObjectTemplate
}
