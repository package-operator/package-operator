package packagedeploy

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type (
	GenericClusterObjectSetList struct {
		corev1alpha1.ClusterObjectSetList
	}

	GenericObjectSetList struct{ corev1alpha1.ObjectSetList }
)

type (
	genericObjectSetList interface {
		ClientObjectList() client.ObjectList
		GetItems() []genericObjectSet
	}

	genericObjectSetListFactory func(scheme *runtime.Scheme) genericObjectSetList
)

var (
	objectSetListGVK        = corev1alpha1.GroupVersion.WithKind("ObjectSetList")
	clusterObjectSetListGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectSetList")

	_ genericObjectSetList = (*GenericObjectSetList)(nil)
	_ genericObjectSetList = (*GenericClusterObjectSetList)(nil)
)

func newGenericObjectSetList(scheme *runtime.Scheme) genericObjectSetList {
	obj, err := scheme.New(objectSetListGVK)
	if err != nil {
		panic(err)
	}

	return &GenericObjectSetList{
		ObjectSetList: *obj.(*corev1alpha1.ObjectSetList),
	}
}

func newGenericClusterObjectSetList(scheme *runtime.Scheme) genericObjectSetList {
	obj, err := scheme.New(clusterObjectSetListGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterObjectSetList{
		ClusterObjectSetList: *obj.(*corev1alpha1.ClusterObjectSetList),
	}
}

func (a *GenericObjectSetList) ClientObjectList() client.ObjectList {
	return &a.ObjectSetList
}

func (a *GenericObjectSetList) GetItems() []genericObjectSet {
	out := make([]genericObjectSet, len(a.Items))
	for i := range a.Items {
		out[i] = &GenericObjectSet{
			ObjectSet: a.Items[i],
		}
	}
	return out
}

func (a *GenericClusterObjectSetList) ClientObjectList() client.ObjectList {
	return &a.ClusterObjectSetList
}

func (a *GenericClusterObjectSetList) GetItems() []genericObjectSet {
	out := make([]genericObjectSet, len(a.Items))
	for i := range a.Items {
		out[i] = &GenericClusterObjectSet{
			ClusterObjectSet: a.Items[i],
		}
	}
	return out
}
