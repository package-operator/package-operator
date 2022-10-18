package objectdeployments

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type genericObjectDeployment interface {
	ClientObject() client.Object
	UpdatePhase()
	GetConditions() *[]metav1.Condition
	GetSelector() metav1.LabelSelector
	GetObjectSetTemplate() corev1alpha1.ObjectSetTemplate
	GetRevisionHistoryLimit() *int32
	SetStatusCollisionCount(*int32)
	GetStatusCollisionCount() *int32
	GetStatusTemplateHash() string
	SetStatusTemplateHash(templateHash string)
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

func (a *GenericObjectDeployment) GetRevisionHistoryLimit() *int32 {
	return a.Spec.RevisionHistoryLimit
}

func (a *GenericObjectDeployment) SetStatusCollisionCount(cc *int32) {
	a.Status.CollisionCount = cc
}

func (a *GenericObjectDeployment) GetStatusCollisionCount() *int32 {
	return a.Status.CollisionCount
}

func (a *GenericObjectDeployment) ClientObject() client.Object {
	return &a.ObjectDeployment
}

func (a *GenericObjectDeployment) UpdatePhase() {
	availableCond := meta.FindStatusCondition(
		a.Status.Conditions,
		corev1alpha1.ObjectDeploymentAvailable)

	if availableCond != nil {
		if availableCond.Status == metav1.ConditionTrue {
			a.Status.Phase = corev1alpha1.ObjectDeploymentPhaseAvailable
			return
		}
		if availableCond.Status == metav1.ConditionFalse {
			a.Status.Phase = corev1alpha1.ObjectDeploymentPhaseNotReady
			return
		}
	}
	a.Status.Phase = corev1alpha1.ObjectDeploymentPhaseProgressing
}

func (a *GenericObjectDeployment) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericObjectDeployment) GetSelector() metav1.LabelSelector {
	return a.Spec.Selector
}

func (a *GenericObjectDeployment) GetObjectSetTemplate() corev1alpha1.ObjectSetTemplate {
	return a.Spec.Template
}

func (a *GenericObjectDeployment) SetStatusTemplateHash(templateHash string) {
	a.Status.TemplateHash = templateHash
}

func (a *GenericObjectDeployment) GetStatusTemplateHash() string {
	return a.Status.TemplateHash
}

type GenericClusterObjectDeployment struct {
	corev1alpha1.ClusterObjectDeployment
}

func (a *GenericClusterObjectDeployment) GetRevisionHistoryLimit() *int32 {
	return a.Spec.RevisionHistoryLimit
}

func (a *GenericClusterObjectDeployment) SetStatusCollisionCount(cc *int32) {
	a.Status.CollisionCount = cc
}

func (a *GenericClusterObjectDeployment) ClientObject() client.Object {
	return &a.ClusterObjectDeployment
}

func (a *GenericClusterObjectDeployment) UpdatePhase() {
	availableCond := meta.FindStatusCondition(
		a.Status.Conditions,
		corev1alpha1.ObjectDeploymentAvailable)

	if availableCond != nil {
		if availableCond.Status == metav1.ConditionTrue {
			a.Status.Phase = corev1alpha1.ObjectDeploymentPhaseAvailable
			return
		}
		if availableCond.Status == metav1.ConditionFalse {
			a.Status.Phase = corev1alpha1.ObjectDeploymentPhaseNotReady
			return
		}
	}
	a.Status.Phase = corev1alpha1.ObjectDeploymentPhaseProgressing
}

func (a *GenericClusterObjectDeployment) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericClusterObjectDeployment) GetSelector() metav1.LabelSelector {
	return a.Spec.Selector
}

func (a *GenericClusterObjectDeployment) GetObjectSetTemplate() corev1alpha1.ObjectSetTemplate {
	return a.Spec.Template
}

func (a *GenericClusterObjectDeployment) GetStatusCollisionCount() *int32 {
	return a.Status.CollisionCount
}

func (a *GenericClusterObjectDeployment) SetStatusTemplateHash(templateHash string) {
	a.Status.TemplateHash = templateHash
}

func (a *GenericClusterObjectDeployment) GetStatusTemplateHash() string {
	return a.Status.TemplateHash
}
