package adapters

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type ObjectSetListAccessor interface {
	ClientObjectList() client.ObjectList
	GetItems() []ObjectSetAccessor
}

type ObjectSetListAccessorFactory func(
	scheme *runtime.Scheme) ObjectSetListAccessor

var (
	objectSetListGVK        = corev1alpha1.GroupVersion.WithKind("ObjectSetList")
	clusterObjectSetListGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectSetList")
)

func NewObjectSetList(scheme *runtime.Scheme) ObjectSetListAccessor {
	obj, err := scheme.New(objectSetListGVK)
	if err != nil {
		panic(err)
	}

	return &ObjectSetList{
		ObjectSetList: *obj.(*corev1alpha1.ObjectSetList),
	}
}

func NewClusterObjectSetList(scheme *runtime.Scheme) ObjectSetListAccessor {
	obj, err := scheme.New(clusterObjectSetListGVK)
	if err != nil {
		panic(err)
	}

	return &ClusterObjectSetList{
		ClusterObjectSetList: *obj.(*corev1alpha1.ClusterObjectSetList),
	}
}

var (
	_ ObjectSetListAccessor = (*ObjectSetList)(nil)
	_ ObjectSetListAccessor = (*ClusterObjectSetList)(nil)
)

type ObjectSetList struct {
	corev1alpha1.ObjectSetList
}

func (a *ObjectSetList) ClientObjectList() client.ObjectList {
	return &a.ObjectSetList
}

func (a *ObjectSetList) GetItems() []ObjectSetAccessor {
	out := make([]ObjectSetAccessor, len(a.Items))
	for i := range a.Items {
		out[i] = &ObjectSetAdapter{
			ObjectSet: a.Items[i],
		}
	}
	return out
}

type ClusterObjectSetList struct {
	corev1alpha1.ClusterObjectSetList
}

func (a *ClusterObjectSetList) ClientObjectList() client.ObjectList {
	return &a.ClusterObjectSetList
}

func (a *ClusterObjectSetList) GetItems() []ObjectSetAccessor {
	out := make([]ObjectSetAccessor, len(a.Items))
	for i := range a.Items {
		out[i] = &ClusterObjectSetAdapter{
			ClusterObjectSet: a.Items[i],
		}
	}
	return out
}
