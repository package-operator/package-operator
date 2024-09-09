package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestObjectSlice(t *testing.T) {
	t.Parallel()
	slice := NewObjectSlice(testScheme).(*ObjectSlice)

	os := slice.ClientObject()
	assert.IsType(t, &corev1alpha1.ObjectSlice{}, os)

	object := []corev1alpha1.ObjectSetObject{}
	slice.SetObjects(object)
	assert.Equal(t, slice.Objects, slice.GetObjects())
}

func TestClusterObjectSlice(t *testing.T) {
	t.Parallel()
	slice := NewClusterObjectSlice(testScheme).(*ClusterObjectSlice)

	cos := slice.ClientObject()
	assert.IsType(t, &corev1alpha1.ClusterObjectSlice{}, cos)

	object := []corev1alpha1.ObjectSetObject{}
	slice.SetObjects(object)
	assert.Equal(t, slice.Objects, slice.GetObjects())
}
