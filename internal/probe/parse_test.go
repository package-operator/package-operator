package probe

import (
	"github.com/stretchr/testify/assert"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
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
		FieldA: p1.FieldsEqual.FieldA,
		FieldB: p1.FieldsEqual.FieldB,
	}
	e2 := &ConditionProbe{
		Type:   p2.Condition.Type,
		Status: p2.Condition.Status,
	}
	e3 := &CurrentGenerationProbe{}

	probeSpecs := []corev1alpha1.Probe{p1, p2, p3, p4}
	parsedProbeSpecs := Parse(probeSpecs)
	expected := ProbeList{e1, e2, e3}
	assert.Equal(t, expected, parsedProbeSpecs)
}
