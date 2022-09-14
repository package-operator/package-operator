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
	kind := "Test"
	group := "test-group"
	osp := []corev1alpha1.ObjectSetProbe{
		{
			Selector: corev1alpha1.ProbeSelector{
				Kind: &corev1alpha1.PackageProbeKindSpec{
					Kind:  kind,
					Group: group,
				},
			},
		},
	}

	p, err := Parse(ctx, osp)
	require.NoError(t, err)
	require.IsType(t, list{}, p)

	if assert.Len(t, p, 1) {
		list := p.(list)
		require.IsType(t, &kindSelector{}, list[0])
		ks := list[0].(*kindSelector)
		assert.Equal(t, kind, ks.Kind)
		assert.Equal(t, group, ks.Group)
	}
}

func TestParseSelector(t *testing.T) {
	ctx := context.Background()
	p, err := ParseSelector(ctx, corev1alpha1.ProbeSelector{
		Kind: &corev1alpha1.PackageProbeKindSpec{
			Kind:  "Test",
			Group: "test",
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"test": "test123",
			},
		},
	}, nil)
	require.NoError(t, err)
	require.IsType(t, &selectorSelector{}, p)

	ss := p.(*selectorSelector)
	require.IsType(t, &kindSelector{}, ss.Prober)
}

func TestParseProbes(t *testing.T) {
	fep := corev1alpha1.Probe{
		FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
			FieldA: "asdf",
			FieldB: "jkl;",
		},
	}
	cp := corev1alpha1.Probe{
		Condition: &corev1alpha1.ProbeConditionSpec{
			Type:   "asdf",
			Status: "asdf",
		},
	}
	emptyConfigProbe := corev1alpha1.Probe{}

	p := ParseProbes(context.Background(), []corev1alpha1.Probe{
		fep, cp, emptyConfigProbe,
	})
	// everything should be wrapped
	require.IsType(t, &statusObservedGenerationProbe{}, p)

	ogProbe := p.(*statusObservedGenerationProbe)
	nested := ogProbe.Prober
	require.IsType(t, list{}, nested)

	if assert.Len(t, nested, 2) {
		nestedList := nested.(list)
		assert.Equal(t, &fieldsEqualProbe{
			FieldA: "asdf",
			FieldB: "jkl;",
		}, nestedList[0])
		assert.Equal(t, &conditionProbe{
			Type:   "asdf",
			Status: "asdf",
		}, nestedList[1])
	}
}
