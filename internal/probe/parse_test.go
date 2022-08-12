package probe

import (
	"github.com/stretchr/testify/assert"
	corev1alpha1 "package-operator.run/package-operator/apis/core/v1alpha1"
	// corev1alpha1 "package-operator.run/package-operator/apis"
	"testing"
)

func TestParse(t *testing.T) {

	p1 := corev1alpha1.Probe{
		FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
			FieldA: "asdf",
			FieldB: "jkl;",
		},
	}
	p2 := corev1alpha1.Probe{
		Condition: &corev1alpha1.ProbeConditionSpec{
			Type:   "asdf",
			Status: "asdf",
		},
	}
	p3 := corev1alpha1.Probe{
		CurrentGeneration: &corev1alpha1.ProbeCurrentGeneration{},
	}
	p4 := corev1alpha1.Probe{}

	e1 := &FieldsEqualProbe{
		FieldA: "asdf",
		FieldB: "jkl;",
	}
	e2 := &ConditionProbe{
		Type:   "asdf",
		Status: "asdf",
	}
	e3 := &CurrentGenerationProbe{}

	probeSpecs := []corev1alpha1.Probe{p1, p2, p3, p4}
	parsedProbeSpecs := Parse(probeSpecs)
	expected := ProbeList{e1, e2, e3}
	assert.Equal(t, expected, parsedProbeSpecs)
}
