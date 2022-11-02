package objectsets

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type genericObjectSlice interface {
	ClientObject() client.Object
	GetObjects() []corev1alpha1.ObjectSetObject
}

type genericObjectSliceFactory func(
	scheme *runtime.Scheme) genericObjectSlice

var (
	objectSliceGVK        = corev1alpha1.GroupVersion.WithKind("ObjectSlice")
	clusterObjectSliceGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectSlice")
)

func newGenericObjectSlice(scheme *runtime.Scheme) genericObjectSlice {
	obj, err := scheme.New(objectSliceGVK)
	if err != nil {
		panic(err)
	}

	return &GenericObjectSlice{
		ObjectSlice: *obj.(*corev1alpha1.ObjectSlice)}
}

func newGenericClusterObjectSlice(scheme *runtime.Scheme) genericObjectSlice {
	obj, err := scheme.New(clusterObjectSliceGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterObjectSlice{
		ClusterObjectSlice: *obj.(*corev1alpha1.ClusterObjectSlice)}
}

var (
	_ genericObjectSlice = (*GenericObjectSlice)(nil)
	_ genericObjectSlice = (*GenericClusterObjectSlice)(nil)
)

type GenericObjectSlice struct {
	corev1alpha1.ObjectSlice
}

func (a *GenericObjectSlice) ClientObject() client.Object {
	return &a.ObjectSlice
}

func (a *GenericObjectSlice) GetObjects() []corev1alpha1.ObjectSetObject {
	return a.Objects
}

type GenericClusterObjectSlice struct {
	corev1alpha1.ClusterObjectSlice
}

func (a *GenericClusterObjectSlice) ClientObject() client.Object {
	return &a.ClusterObjectSlice
}

func (a *GenericClusterObjectSlice) GetObjects() []corev1alpha1.ObjectSetObject {
	return a.Objects
}
