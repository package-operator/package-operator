package probe

import (
	packagesv1alpha1 "package-operator.run/package-operator/apis/core/v1alpha1"
)

func Parse(probeSpecs []packagesv1alpha1.Probe) ProbeInterface {
	var probeList ProbeList
	for _, probeSpec := range probeSpecs {
		// main probe type
		var probe ProbeInterface

		switch {
		case probeSpec.FieldsEqual == nil:
			probe = &FieldsEqualProbe{
				FieldA: probeSpec.FieldsEqual.FieldA,
				FieldB: probeSpec.FieldsEqual.FieldB,
			}
		case probeSpec.Condition == nil:
			probe = &ConditionProbe{
				Type:   probeSpec.Condition.Type,
				Status: probeSpec.Condition.Status,
			}
		case probeSpec.CurrentGeneration != nil:
			probe = &CurrentGenerationProbe{}
		default:
			// Unknown probe type
			panic("unknown probe type")
			// continue
		}
		probeList = append(probeList, probe)
	}

	return probeList
}
