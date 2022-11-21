package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestObjectSliceList(t *testing.T) {
	sliceList := NewObjectSliceList(testScheme).(*ObjectSliceList)
	assert.IsType(t, &corev1alpha1.ObjectSliceList{}, sliceList.ClientObjectList())

	sliceList.Items = []corev1alpha1.ObjectSlice{
		{
			ObjectMeta: metav1.ObjectMeta{},
		},
	}
	items := sliceList.GetItems()
	if assert.Len(t, items, 1) {
		assert.IsType(t, &ObjectSlice{}, items[0])
	}
}

func TestClusterObjectSliceList(t *testing.T) {
	sliceList := NewClusterObjectSliceList(testScheme).(*ClusterObjectSliceList)
	assert.IsType(t, &corev1alpha1.ClusterObjectSliceList{}, sliceList.ClientObjectList())

	sliceList.Items = []corev1alpha1.ClusterObjectSlice{
		{
			ObjectMeta: metav1.ObjectMeta{},
		},
	}
	items := sliceList.GetItems()
	if assert.Len(t, items, 1) {
		assert.IsType(t, &ClusterObjectSlice{}, items[0])
	}
}
