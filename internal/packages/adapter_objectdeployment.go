package packages

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type genericObjectDeployment interface {
	ClientObject() client.Object
	SetTemplateSpec(corev1alpha1.ObjectSetTemplateSpec)
	GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec
	SetSelector(labels map[string]string)
	GetSelector() metav1.LabelSelector
}

type genericObjectDeploymentFactory func(
	scheme *runtime.Scheme) genericObjectDeployment

var (
	objectDeploymentGVK        = corev1alpha1.GroupVersion.WithKind("ObjectDeployment")
	clusterObjectDeploymentGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectDeployment")
)

func newGenericObjectDeployment(scheme *runtime.Scheme) genericObjectDeployment {
	obj, err := scheme.New(objectDeploymentGVK)
	if err != nil {
		panic(err)
	}

	return &GenericObjectDeployment{
		ObjectDeployment: *obj.(*corev1alpha1.ObjectDeployment)}
}

func newGenericClusterObjectDeployment(scheme *runtime.Scheme) genericObjectDeployment {
	obj, err := scheme.New(clusterObjectDeploymentGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterObjectDeployment{
		ClusterObjectDeployment: *obj.(*corev1alpha1.ClusterObjectDeployment)}
}

var (
	_ genericObjectDeployment = (*GenericObjectDeployment)(nil)
	_ genericObjectDeployment = (*GenericClusterObjectDeployment)(nil)
)

type GenericObjectDeployment struct {
	corev1alpha1.ObjectDeployment
}

func (a *GenericObjectDeployment) ClientObject() client.Object {
	return &a.ObjectDeployment
}

func (a *GenericObjectDeployment) SetTemplateSpec(spec corev1alpha1.ObjectSetTemplateSpec) {
	a.Spec.Template.Spec = spec
}

func (a *GenericObjectDeployment) GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	return a.Spec.Template.Spec
}

func (a *GenericObjectDeployment) SetSelector(labels map[string]string) {
	a.Spec.Selector = metav1.LabelSelector{
		MatchLabels: labels,
	}
	a.Spec.Template.Metadata.Labels = labels
}

func (a *GenericObjectDeployment) GetSelector() metav1.LabelSelector {
	return a.Spec.Selector
}

type GenericClusterObjectDeployment struct {
	corev1alpha1.ClusterObjectDeployment
}

func (a *GenericClusterObjectDeployment) ClientObject() client.Object {
	return &a.ClusterObjectDeployment
}

func (a *GenericClusterObjectDeployment) SetTemplateSpec(spec corev1alpha1.ObjectSetTemplateSpec) {
	a.Spec.Template.Spec = spec
}

func (a *GenericClusterObjectDeployment) GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	return a.Spec.Template.Spec
}

func (a *GenericClusterObjectDeployment) SetSelector(labels map[string]string) {
	a.Spec.Selector = metav1.LabelSelector{
		MatchLabels: labels,
	}
	a.Spec.Template.Metadata.Labels = labels
}

func (a *GenericClusterObjectDeployment) GetSelector() metav1.LabelSelector {
	return a.Spec.Selector
}
