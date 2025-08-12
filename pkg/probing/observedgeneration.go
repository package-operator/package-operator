package probing

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"pkg.package-operator.run/boxcutter/machinery/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObservedGenerationProbe wraps the given Prober and ensures that .status.observedGeneration is equal to
// .metadata.generation, before running the given probe. If the probed object does not contain the
// .status.observedGeneration field, the given prober is executed directly.
type ObservedGenerationProbe struct {
	Prober
}

var _ Prober = (*ObservedGenerationProbe)(nil)

// Probe executes the probe.
func (cg *ObservedGenerationProbe) Probe(obj client.Object) types.ProbeResult {
	unstr := toUnstructured(obj)
	if observedGeneration, ok, err := unstructured.NestedInt64(
		unstr.Object, "status", "observedGeneration",
	); err == nil && ok && observedGeneration != obj.GetGeneration() {
		return types.ProbeResult{
			Status:   types.ProbeStatusFalse,
			Messages: []string{".status outdated"},
		}
	}
	return cg.Prober.Probe(obj)
}
