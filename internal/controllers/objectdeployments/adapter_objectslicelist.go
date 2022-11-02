package objectdeployments

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type genericObjectSliceList interface {
	ClientObjectList() client.ObjectList
	GetItems() []genericObjectSlice
}

type genericObjectSliceListFactory func(
	scheme *runtime.Scheme) genericObjectSliceList

var (
	objectSliceListGVK        = corev1alpha1.GroupVersion.WithKind("ObjectSliceList")
	clusterObjectSliceListGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectSliceList")
)

func newGenericObjectSliceList(scheme *runtime.Scheme) genericObjectSliceList {
	obj, err := scheme.New(objectSliceListGVK)
	if err != nil {
		panic(err)
	}

	return &GenericObjectSliceList{
		ObjectSliceList: *obj.(*corev1alpha1.ObjectSliceList)}
}

func newGenericClusterObjectSliceList(scheme *runtime.Scheme) genericObjectSliceList {
	obj, err := scheme.New(clusterObjectSliceListGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterObjectSliceList{
		ClusterObjectSliceList: *obj.(*corev1alpha1.ClusterObjectSliceList)}
}

var (
	_ genericObjectSliceList = (*GenericObjectSliceList)(nil)
	_ genericObjectSliceList = (*GenericClusterObjectSliceList)(nil)
)

type GenericObjectSliceList struct {
	corev1alpha1.ObjectSliceList
}

func (a *GenericObjectSliceList) ClientObjectList() client.ObjectList {
	return &a.ObjectSliceList
}

func (a *GenericObjectSliceList) GetItems() []genericObjectSlice {
	out := make([]genericObjectSlice, len(a.Items))
	for i := range a.Items {
		out[i] = &GenericObjectSlice{
			ObjectSlice: a.Items[i],
		}
	}
	return out
}

type GenericClusterObjectSliceList struct {
	corev1alpha1.ClusterObjectSliceList
}

func (a *GenericClusterObjectSliceList) ClientObjectList() client.ObjectList {
	return &a.ClusterObjectSliceList
}

func (a *GenericClusterObjectSliceList) GetItems() []genericObjectSlice {
	out := make([]genericObjectSlice, len(a.Items))
	for i := range a.Items {
		out[i] = &GenericClusterObjectSlice{
			ClusterObjectSlice: a.Items[i],
		}
	}
	return out
}
