package probing

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"pkg.package-operator.run/boxcutter/probing"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// Parse takes a list of ObjectSetProbes (commonly defined within a ObjectSetPhaseSpec)
// and compiles a single Prober to test objects with.
func Parse(ctx context.Context, packageProbes []corev1alpha1.ObjectSetProbe) (probing.Prober, error) {
	probeList := make(probing.And, len(packageProbes))
	for i, pkgProbe := range packageProbes {
		var (
			probe probing.Prober
			err   error
		)
		probe, err = ParseProbes(ctx, pkgProbe.Probes)
		if err != nil {
			return nil, fmt.Errorf("parsing probe #%d: %w", i, err)
		}
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
func ParseSelector(
	_ context.Context, selector corev1alpha1.ProbeSelector, probe probing.Prober,
) (probing.Prober, error) {
	if selector.Kind != nil {
		probe = &probing.GroupKindSelector{
			Prober: probe,
			GroupKind: schema.GroupKind{
				Group: selector.Kind.Group,
				Kind:  selector.Kind.Kind,
			},
		}
	}
	if selector.Selector != nil {
		s, err := metav1.LabelSelectorAsSelector(selector.Selector)
		if err != nil {
			return nil, err
		}
		probe = &probing.LabelSelector{
			Prober:   probe,
			Selector: s,
		}
	}
	return probe, nil
}

// ParseProbes takes a []corev1alpha1.Probe and compiles it into a Prober.
func ParseProbes(_ context.Context, probeSpecs []corev1alpha1.Probe) (probing.Prober, error) {
	var probeList probing.And
	for _, probeSpec := range probeSpecs {
		var (
			probe probing.Prober
			err   error
		)

		switch {
		case probeSpec.FieldsEqual != nil:
			probe = &probing.FieldsEqualProbe{
				FieldA: probeSpec.FieldsEqual.FieldA,
				FieldB: probeSpec.FieldsEqual.FieldB,
			}

		case probeSpec.Condition != nil:
			probe = &probing.ConditionProbe{
				Type:   probeSpec.Condition.Type,
				Status: probeSpec.Condition.Status,
			}

		case probeSpec.CEL != nil:
			probe, err = probing.NewCELProbe(
				probeSpec.CEL.Rule,
				probeSpec.CEL.Message,
			)
			if err != nil {
				return nil, err
			}

		default:
			// probe has no known config
			continue
		}
		probeList = append(probeList, probe)
	}

	// Always check .status.observedCondition, if present.
	return &probing.ObservedGenerationProbe{Prober: probeList}, nil
}
