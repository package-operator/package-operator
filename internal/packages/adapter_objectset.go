package packages

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type genericObjectSet interface {
	ClientObject() client.Object
	GetPhases() []corev1alpha1.ObjectSetTemplatePhase
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

func (a *GenericObjectSet) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.ObjectSetTemplateSpec.Phases
}

type GenericClusterObjectSet struct {
	corev1alpha1.ClusterObjectSet
}

func (a *GenericClusterObjectSet) ClientObject() client.Object {
	return &a.ClusterObjectSet
}

func (a *GenericClusterObjectSet) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.Phases
}
