package probing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

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
	require.IsType(t, list{}, p)

	if assert.Len(t, p, 1) {
		list := p.(list)
		assert.IsType(t, &kindSelector{}, list[0])
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
	require.IsType(t, &selectorSelector{}, p)

	ss := p.(*selectorSelector)
	require.IsType(t, &kindSelector{}, ss.Prober)
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
	require.IsType(t, &statusObservedGeneration{}, p)

	ogProbe := p.(*statusObservedGeneration)
	nested := ogProbe.Prober
	require.IsType(t, list{}, nested)

	if assert.Len(t, nested, 2) {
		nestedList := nested.(list)
		assert.Equal(t, &fieldsEqual{
			FieldA: "asdf",
			FieldB: "jkl;",
		}, nestedList[0])
		assert.Equal(t, &condition{
			Type:   "asdf",
			Status: "asdf",
		}, nestedList[1])
	}
}
