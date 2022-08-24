package probing

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// Parse takes a list of ObjectSetProbes (commonly defined within a ObjectSetPhaseSpec)
// and compiles a single Prober to test objects with.
func Parse(ctx context.Context, packageProbes []corev1alpha1.ObjectSetProbe) (Prober, error) {
	probeList := make(list, len(packageProbes))
	for i, pkgProbe := range packageProbes {
		probe := ParseProbes(ctx, pkgProbe.Probes)
		var err error
		probe, err = ParseSelector(ctx, pkgProbe.Selector, probe)
		if err != nil {
			return nil, fmt.Errorf("parsing selector of probe #%d: %w", i, err)
		}
		probeList[i] = probe
	}
	return probeList, nil
}

// ParseSelector reads a corev1alpha1.ProbeSelector and wraps a Prober,
// only executing the Prober when the selector criteria match.
func ParseSelector(ctx context.Context, selector corev1alpha1.ProbeSelector, probe Prober) (Prober, error) {
	if selector.Kind != nil {
		probe = &kindSelector{
			Prober: probe,
			GroupKind: schema.GroupKind{
				Group: selector.Kind.Group,
				Kind:  selector.Kind.Kind,
			},
		}
	}
	if selector.Selector != nil {
		s, err := metav1.LabelSelectorAsSelector(&selector.Selector.Selector)
		if err != nil {
			return nil, err
		}
		probe = &selectorSelector{
			Prober:   probe,
			Selector: s,
		}
	}
	return probe, nil
}

// ParseProbes takes a []corev1alpha1.Probe and compiles it into a Prober.
func ParseProbes(ctx context.Context, probeSpecs []corev1alpha1.Probe) Prober {
	var probeList list
	for _, probeSpec := range probeSpecs {
		var probe Prober

		switch {
		case probeSpec.FieldsEqual != nil:
			probe = &fieldsEqualProbe{
				FieldA: probeSpec.FieldsEqual.FieldA,
				FieldB: probeSpec.FieldsEqual.FieldB,
			}

		case probeSpec.Condition != nil:
			probe = &conditionProbe{
				Type:   probeSpec.Condition.Type,
				Status: probeSpec.Condition.Status,
			}

		default:
			// probe has no known config
			continue
		}
		probeList = append(probeList, probe)
	}

	// Always check .status.observedCondition, if present.
	return &statusObservedGenerationProbe{Prober: probeList}
}
