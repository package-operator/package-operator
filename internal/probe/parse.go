package probe

import (
	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
)

func Parse(probeSpecs []packagesv1alpha1.Probe) Interface {
	var probeList ProbeList
	for _, probeSpec := range probeSpecs {
		// main probe type
		var probe Interface
		switch probeSpec.Type {
		case packagesv1alpha1.ProbeCondition:
			if probeSpec.Condition == nil {
				continue
			}

			probe = &ConditionProbe{
				Type:   probeSpec.Condition.Type,
				Status: probeSpec.Condition.Status,
			}

		case packagesv1alpha1.ProbeFieldsEqual:
			if probeSpec.FieldsEqual == nil {
				continue
			}

			probe = &FieldsEqualProbe{
				FieldA: probeSpec.FieldsEqual.FieldA,
				FieldB: probeSpec.FieldsEqual.FieldB,
			}

		default:
			// Unknown probe type
			panic("unknown probe type")
			// continue
		}

		probeList = append(probeList, probe)
	}

	return probeList
}
