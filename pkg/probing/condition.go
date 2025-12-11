package probing

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"pkg.package-operator.run/boxcutter/machinery/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConditionProbe checks if the object's condition is set and in a certain status.
type ConditionProbe struct {
	Type, Status string
}

var _ Prober = (*ConditionProbe)(nil)

// Probe executes the probe.
func (cp *ConditionProbe) Probe(obj client.Object) types.ProbeResult {
	return probeUnstructuredSingleMsg(obj, cp.probe)
}

func (cp *ConditionProbe) probe(obj *unstructured.Unstructured) (success bool, message string) {
	defer func() {
		if success {
			return
		}
		// add probed condition type and status as context to error message.
		message = fmt.Sprintf("condition %q == %q: %s", cp.Type, cp.Status, message)
	}()

	rawConditions, exist, err := unstructured.NestedFieldNoCopy(
		obj.Object, "status", "conditions")
	conditions, ok := rawConditions.([]any)
	if err != nil || !exist {
		return false, "missing .status.conditions"
	}
	if !ok {
		return false, "malformed"
	}

	for _, condI := range conditions {
		cond, ok := condI.(map[string]any)
		if !ok {
			// no idea what this is supposed to be
			return false, "malformed"
		}

		if cond["type"] != cp.Type {
			// not the type we are probing for
			continue
		}

		// Check the condition's observed generation, if set
		if observedGeneration, ok, err := unstructured.NestedInt64(
			cond, "observedGeneration",
		); err == nil && ok && observedGeneration != obj.GetGeneration() {
			return false, "outdated"
		}

		if cond["status"] == cp.Status {
			return true, ""
		}
		return false, "wrong status"
	}
	return false, "not reported"
}
