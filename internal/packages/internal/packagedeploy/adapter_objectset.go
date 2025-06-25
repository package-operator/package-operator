package packagedeploy

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type (
	GenericObjectSet        struct{ corev1alpha1.ObjectSet }
	GenericClusterObjectSet struct{ corev1alpha1.ClusterObjectSet }
)

type genericObjectSet interface {
	ClientObject() client.Object
	GetSpecPhases() []corev1alpha1.ObjectSetTemplatePhase
}

var (
	_ genericObjectSet = (*GenericObjectSet)(nil)
	_ genericObjectSet = (*GenericClusterObjectSet)(nil)
)

func (a *GenericObjectSet) ClientObject() client.Object        { return &a.ObjectSet }
func (a *GenericClusterObjectSet) ClientObject() client.Object { return &a.ClusterObjectSet }

func (a *GenericObjectSet) GetSpecPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.Phases
}

func (a *GenericClusterObjectSet) GetSpecPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.Phases
}
