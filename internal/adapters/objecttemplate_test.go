package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestGenericObjectTemplate(t *testing.T) {
	t.Parallel()

	ot := NewGenericObjectTemplate(testScheme).(*GenericObjectTemplate)

	co := ot.ClientObject()
	assert.IsType(t, &corev1alpha1.ObjectTemplate{}, co)

	var generation int64 = 2
	ot.Generation = generation
	assert.Equal(t, generation, ot.GetGeneration())

	controlledObj := corev1alpha1.ControlledObjectReference{}
	ot.SetStatusControllerOf(controlledObj)
	assert.Equal(t, controlledObj, ot.GetStatusControllerOf())

	ot.Status.Conditions = []metav1.Condition{}
	assert.Equal(t, ot.Status.Conditions, *ot.GetConditions())

	sources := []corev1alpha1.ObjectTemplateSource{}
	ot.Spec.Sources = sources
	assert.Equal(t, sources, ot.GetSources())

	ot.Spec.Template = ""
	assert.Equal(t, ot.Spec.Template, ot.GetTemplate())
}

func TestGenericClusterObjectTemplate(t *testing.T) {
	t.Parallel()

	ot := NewGenericClusterObjectTemplate(testScheme).(*GenericClusterObjectTemplate)

	co := ot.ClientObject()
	assert.IsType(t, &corev1alpha1.ClusterObjectTemplate{}, co)

	var generation int64 = 2
	ot.Generation = generation
	assert.Equal(t, generation, ot.GetGeneration())

	controlledObj := corev1alpha1.ControlledObjectReference{}
	ot.SetStatusControllerOf(controlledObj)
	assert.Equal(t, controlledObj, ot.GetStatusControllerOf())

	ot.Status.Conditions = []metav1.Condition{}
	assert.Equal(t, ot.Status.Conditions, *ot.GetConditions())

	sources := []corev1alpha1.ObjectTemplateSource{}
	ot.Spec.Sources = sources
	assert.Equal(t, sources, ot.GetSources())

	ot.Spec.Template = ""
	assert.Equal(t, ot.Spec.Template, ot.GetTemplate())
}
