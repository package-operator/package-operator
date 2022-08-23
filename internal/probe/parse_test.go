package probe

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// func TestParse(t *testing.T) {

// 	p1 := corev1alpha1.Probe{
// 		FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
// 			FieldA: "asdf",
// 			FieldB: "jkl;",
// 		},
// 	}
// 	p2 := corev1alpha1.Probe{
// 		Condition: &corev1alpha1.ProbeConditionSpec{
// 			Type:   "asdf",
// 			Status: "asdf",
// 		},
// 	}
// 	p3 := corev1alpha1.Probe{
// 		CurrentGeneration: &corev1alpha1.ProbeCurrentGeneration{},
// 	}
// 	p4 := corev1alpha1.Probe{}

// 	e1 := &FieldsEqualProbe{
// 		FieldA: p1.FieldsEqual.FieldA,
// 		FieldB: p1.FieldsEqual.FieldB,
// 	}
// 	e2 := &ConditionProbe{
// 		Type:   p2.Condition.Type,
// 		Status: p2.Condition.Status,
// 	}
// 	e3 := &CurrentGenerationProbe{}

// 	probeSpecs := []corev1alpha1.Probe{p1, p2, p3, p4}
// 	ctx := context.Background()
// 	parsedProbeSpecs := Parse(ctx, probeSpecs)
// 	expected := ProbeList{e1, e2, e3}
// 	assert.Equal(t, expected, parsedProbeSpecs)
// }

func TestParse(t *testing.T) {
	ctx := context.Background()
	osp := []corev1alpha1.ObjectSetProbe{
		{
			Selector: corev1alpha1.ProbeSelector{
				Kind: &corev1alpha1.PackageProbeKindSpec{
					Kind:  "Test",
					Group: "test",
				},
			},
		},
	}

	p, err := Parse(ctx, osp)
	require.NoError(t, err)
	require.IsType(t, List{}, p)

	if assert.Len(t, p, 1) {
		list := p.(List)
		assert.IsType(t, &KindSelector{}, list[0])
	}
}

func TestParseSelector(t *testing.T) {
	ctx := context.Background()
	p, err := ParseSelector(ctx, corev1alpha1.ProbeSelector{
		Kind: &corev1alpha1.PackageProbeKindSpec{
			Kind:  "Test",
			Group: "test",
		},
		Selector: &corev1alpha1.PackageProbeSelectorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test": "test123",
				},
			},
		},
	}, nil)
	require.NoError(t, err)
	require.IsType(t, &SelectorSelector{}, p)

	ss := p.(*SelectorSelector)
	require.IsType(t, &KindSelector{}, ss.Prober)
}

func TestParseProbes(t *testing.T) {
	fieldsEqualProbe := corev1alpha1.Probe{
		FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
			FieldA: "asdf",
			FieldB: "jkl;",
		},
	}
	conditionProbe := corev1alpha1.Probe{
		Condition: &corev1alpha1.ProbeConditionSpec{
			Type:   "asdf",
			Status: "asdf",
		},
	}
	emptyConfigProbe := corev1alpha1.Probe{}

	p := ParseProbes(context.Background(), []corev1alpha1.Probe{
		fieldsEqualProbe, conditionProbe, emptyConfigProbe,
	})
	// everything should be wrapped
	require.IsType(t, &StatusObservedGeneration{}, p)

	ogProbe := p.(*StatusObservedGeneration)
	nested := ogProbe.Prober
	require.IsType(t, List{}, nested)

	if assert.Len(t, nested, 2) {
		nestedList := nested.(List)
		assert.Equal(t, &FieldsEqual{
			FieldA: "asdf",
			FieldB: "jkl;",
		}, nestedList[0])
		assert.Equal(t, &Condition{
			Type:   "asdf",
			Status: "asdf",
		}, nestedList[1])
	}
}
