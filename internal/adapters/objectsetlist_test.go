package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestObjectSetList(t *testing.T) {
	t.Parallel()

	objectSetList := NewObjectSetList(testScheme).(*ObjectSetList)

	assert.Equal(t, objectSetList.ObjectSetList, *objectSetList.ClientObjectList().(*corev1alpha1.ObjectSetList))

	os := corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "banana",
		},
	}
	objectSetList.Items = []corev1alpha1.ObjectSet{os}
	items := objectSetList.GetItems()
	assert.Len(t, items, 1)
	assert.Equal(t, os.GetName(), items[0].ClientObject().GetName())

	// Test that GetItems() is not a shallow copy
	items[0].ClientObject().SetName("not-banana")
	items = objectSetList.GetItems()
	assert.Len(t, items, 1)
	assert.Equal(t, os.GetName(), items[0].ClientObject().GetName())
}

func TestClusterObjectSetList(t *testing.T) {
	t.Parallel()

	objectSetList := NewClusterObjectSetList(testScheme).(*ClusterObjectSetList)

	assert.Equal(t, objectSetList.ClusterObjectSetList,
		*objectSetList.ClientObjectList().(*corev1alpha1.ClusterObjectSetList))

	os := corev1alpha1.ClusterObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "banana",
		},
	}
	objectSetList.Items = []corev1alpha1.ClusterObjectSet{os}
	items := objectSetList.GetItems()
	assert.Len(t, items, 1)
	assert.Equal(t, os.GetName(), items[0].ClientObject().GetName())

	// Test that GetItems() is not a shallow copy
	items[0].ClientObject().SetName("not-banana")
	items = objectSetList.GetItems()
	assert.Len(t, items, 1)
	assert.Equal(t, os.GetName(), items[0].ClientObject().GetName())
}
