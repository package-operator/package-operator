package objectdeployments

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type genericObjectSlice interface {
	ClientObject() client.Object
	GetObjects() []corev1alpha1.ObjectSetObject
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
