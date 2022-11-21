package adapters

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type ObjectSliceListAccessor interface {
	ClientObjectList() client.ObjectList
	GetItems() []ObjectSliceAccessor
}

type ObjectSliceListFactory func(
	scheme *runtime.Scheme) ObjectSliceListAccessor

var (
	objectSliceListGVK        = corev1alpha1.GroupVersion.WithKind("ObjectSliceList")
	clusterObjectSliceListGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectSliceList")
)

func NewObjectSliceList(scheme *runtime.Scheme) ObjectSliceListAccessor {
	obj, err := scheme.New(objectSliceListGVK)
	if err != nil {
		panic(err)
	}

	return &ObjectSliceList{
		ObjectSliceList: *obj.(*corev1alpha1.ObjectSliceList)}
}

func NewClusterObjectSliceList(scheme *runtime.Scheme) ObjectSliceListAccessor {
	obj, err := scheme.New(clusterObjectSliceListGVK)
	if err != nil {
		panic(err)
	}

	return &ClusterObjectSliceList{
		ClusterObjectSliceList: *obj.(*corev1alpha1.ClusterObjectSliceList)}
}

var (
	_ ObjectSliceListAccessor = (*ObjectSliceList)(nil)
	_ ObjectSliceListAccessor = (*ClusterObjectSliceList)(nil)
)

type ObjectSliceList struct {
	corev1alpha1.ObjectSliceList
}

func (a *ObjectSliceList) ClientObjectList() client.ObjectList {
	return &a.ObjectSliceList
}

func (a *ObjectSliceList) GetItems() []ObjectSliceAccessor {
	out := make([]ObjectSliceAccessor, len(a.Items))
	for i := range a.Items {
		out[i] = &ObjectSlice{
			ObjectSlice: a.Items[i],
		}
	}
	return out
}

type ClusterObjectSliceList struct {
	corev1alpha1.ClusterObjectSliceList
}

func (a *ClusterObjectSliceList) ClientObjectList() client.ObjectList {
	return &a.ClusterObjectSliceList
}

func (a *ClusterObjectSliceList) GetItems() []ObjectSliceAccessor {
	out := make([]ObjectSliceAccessor, len(a.Items))
	for i := range a.Items {
		out[i] = &ClusterObjectSlice{
			ClusterObjectSlice: a.Items[i],
		}
	}
	return out
}
