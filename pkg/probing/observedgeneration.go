package probing

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// ObservedGenerationProbe wraps the given Prober and ensures that .status.observedGeneration is equal to .metadata.generation,
// before running the given probe. If the probed object does not contain the .status.observedGeneration field,
// the given prober is executed directly.
type ObservedGenerationProbe struct {
	Prober
}

var _ Prober = (*ObservedGenerationProbe)(nil)

func (cg *ObservedGenerationProbe) Probe(obj *unstructured.Unstructured) (success bool, message string) {
	if observedGeneration, ok, err := unstructured.NestedInt64(
		obj.Object, "status", "observedGeneration",
	); err == nil && ok && observedGeneration != obj.GetGeneration() {
		return false, ".status outdated"
	}
	return cg.Prober.Probe(obj)
}
