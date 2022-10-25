package packages

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type genericObjectDeploymentFactory func(scheme *runtime.Scheme) genericObjectDeployment

func newGenericClusterObjectDeployment(scheme *runtime.Scheme) genericObjectDeployment {
	obj, err := scheme.New(clusterObjectDeploymentGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterObjectDeployment{
		ClusterObjectDeployment: *obj.(*corev1alpha1.ClusterObjectDeployment)}
}

func newGenericObjectDeployment(scheme *runtime.Scheme) genericObjectDeployment {
	obj, err := scheme.New(objectDeploymentGVK)
	if err != nil {
		panic(err)
	}

	return &GenericObjectDeployment{
		ObjectDeployment: *obj.(*corev1alpha1.ObjectDeployment)}
}

type genericObjectDeployment interface {
	ClientObject() client.Object
	GetPhases() []corev1alpha1.ObjectSetTemplatePhase
	SetPhases(phases []corev1alpha1.ObjectSetTemplatePhase)
	GetConditions() []metav1.Condition
	GetObjectMeta() metav1.ObjectMeta
	SetObjectMeta(metav1.ObjectMeta)
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

func (a *GenericObjectDeployment) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.Template.Spec.Phases
}

func (a *GenericObjectDeployment) SetPhases(phases []corev1alpha1.ObjectSetTemplatePhase) {
	a.Spec.Template.Spec.Phases = phases
}

func (a *GenericObjectDeployment) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

func (a *GenericObjectDeployment) GetObjectMeta() metav1.ObjectMeta {
	return a.ObjectMeta
}

func (a *GenericObjectDeployment) SetObjectMeta(m metav1.ObjectMeta) {
	a.ObjectMeta = m
}

type GenericClusterObjectDeployment struct {
	corev1alpha1.ClusterObjectDeployment
}

func (a *GenericClusterObjectDeployment) ClientObject() client.Object {
	return &a.ClusterObjectDeployment
}

func (a *GenericClusterObjectDeployment) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.Template.Spec.Phases
}

func (a *GenericClusterObjectDeployment) SetPhases(phases []corev1alpha1.ObjectSetTemplatePhase) {
	a.Spec.Template.Spec.Phases = phases
}

func (a *GenericClusterObjectDeployment) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

func (a *GenericClusterObjectDeployment) GetObjectMeta() metav1.ObjectMeta {
	return a.ObjectMeta
}

func (a *GenericClusterObjectDeployment) SetObjectMeta(m metav1.ObjectMeta) {
	a.ObjectMeta = m
}

var (
	objectDeploymentGVK        = corev1alpha1.GroupVersion.WithKind("ObjectDeployment")
	clusterObjectDeploymentGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectDeployment")
)
