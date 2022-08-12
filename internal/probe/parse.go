package probe

import (
	packagesv1alpha1 "package-operator.run/package-operator/apis/core/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func Parse(probeSpecs []packagesv1alpha1.Probe) ProbeList {
	logger := ctrl.Log.WithName("parse_function") // TODO: I think we should make this into an argument?
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
			logger.Info("No probe given")
			// continue
		}
		probeList = append(probeList, probe)
	}

	return probeList
}
