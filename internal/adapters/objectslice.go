package adapters

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type ObjectSliceAccessor interface {
	ClientObject() client.Object
	GetObjects() []corev1alpha1.ObjectSetObject
	SetObjects([]corev1alpha1.ObjectSetObject)
}

type ObjectSliceFactory func(
	scheme *runtime.Scheme) ObjectSliceAccessor

var (
	objectSliceGVK        = corev1alpha1.GroupVersion.WithKind("ObjectSlice")
	clusterObjectSliceGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectSlice")
)

func NewObjectSlice(scheme *runtime.Scheme) ObjectSliceAccessor {
	obj, err := scheme.New(objectSliceGVK)
	if err != nil {
		panic(err)
	}

	return &ObjectSlice{
		ObjectSlice: *obj.(*corev1alpha1.ObjectSlice)}
}

func NewClusterObjectSlice(scheme *runtime.Scheme) ObjectSliceAccessor {
	obj, err := scheme.New(clusterObjectSliceGVK)
	if err != nil {
		panic(err)
	}

	return &ClusterObjectSlice{
		ClusterObjectSlice: *obj.(*corev1alpha1.ClusterObjectSlice)}
}

var (
	_ ObjectSliceAccessor = (*ObjectSlice)(nil)
	_ ObjectSliceAccessor = (*ClusterObjectSlice)(nil)
)

type ObjectSlice struct {
	corev1alpha1.ObjectSlice
}

func (a *ObjectSlice) ClientObject() client.Object {
	return &a.ObjectSlice
}

func (a *ObjectSlice) GetObjects() []corev1alpha1.ObjectSetObject {
	return a.Objects
}

func (a *ObjectSlice) SetObjects(objects []corev1alpha1.ObjectSetObject) {
	a.Objects = objects
}

type ClusterObjectSlice struct {
	corev1alpha1.ClusterObjectSlice
}

func (a *ClusterObjectSlice) ClientObject() client.Object {
	return &a.ClusterObjectSlice
}

func (a *ClusterObjectSlice) GetObjects() []corev1alpha1.ObjectSetObject {
	return a.Objects
}

func (a *ClusterObjectSlice) SetObjects(objects []corev1alpha1.ObjectSetObject) {
	a.Objects = objects
}
