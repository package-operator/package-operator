package probing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"pkg.package-operator.run/boxcutter/probing"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestParse(t *testing.T) {
	t.Parallel()
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
	require.IsType(t, probing.And{}, p)

	if assert.Len(t, p, 1) {
		list := p.(probing.And)
		require.IsType(t, &probing.GroupKindSelector{}, list[0])
		ks := list[0].(*probing.GroupKindSelector)
		assert.Equal(t, kind, ks.Kind)
		assert.Equal(t, group, ks.Group)
	}
}

func TestParseSelector(t *testing.T) {
	t.Parallel()
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
	require.IsType(t, &probing.LabelSelector{}, p)

	ss := p.(*probing.LabelSelector)
	require.IsType(t, &probing.GroupKindSelector{}, ss.Prober)
}

func TestParseProbes(t *testing.T) {
	t.Parallel()
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
	cel := corev1alpha1.Probe{
		CEL: &corev1alpha1.ProbeCELSpec{
			Message: "test",
			Rule:    `self.metadata.name == "test"`,
		},
	}
	emptyConfigProbe := corev1alpha1.Probe{}

	p, err := ParseProbes(context.Background(), []corev1alpha1.Probe{
		fep, cp, cel, emptyConfigProbe,
	})
	require.NoError(t, err)
	// everything should be wrapped
	require.IsType(t, &probing.ObservedGenerationProbe{}, p)

	ogProbe := p.(*probing.ObservedGenerationProbe)
	nested := ogProbe.Prober
	require.IsType(t, probing.And{}, nested)

	if assert.Len(t, nested, 3) {
		nestedList := nested.(probing.And)
		assert.Equal(t, &probing.FieldsEqualProbe{
			FieldA: "asdf",
			FieldB: "jkl;",
		}, nestedList[0])
		assert.Equal(t, &probing.ConditionProbe{
			Type:   "asdf",
			Status: "asdf",
		}, nestedList[1])
	}
}
