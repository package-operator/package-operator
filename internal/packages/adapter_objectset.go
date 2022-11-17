package packages

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type genericObjectSet interface {
	ClientObject() client.Object
	GetPhases() []corev1alpha1.ObjectSetTemplatePhase
}

type genericObjectSetFactory func(
	scheme *runtime.Scheme) genericObjectSet

var (
	objectSetGVK        = corev1alpha1.GroupVersion.WithKind("ObjectSet")
	clusterObjectSetGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectSet")
)

func newGenericObjectSet(scheme *runtime.Scheme) genericObjectSet {
	obj, err := scheme.New(objectSetGVK)
	if err != nil {
		panic(err)
	}

	return &GenericObjectSet{
		ObjectSet: *obj.(*corev1alpha1.ObjectSet)}
}

func newGenericClusterObjectSet(scheme *runtime.Scheme) genericObjectSet {
	obj, err := scheme.New(clusterObjectSetGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterObjectSet{
		ClusterObjectSet: *obj.(*corev1alpha1.ClusterObjectSet)}
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
