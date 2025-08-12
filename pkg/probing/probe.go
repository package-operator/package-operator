package probing

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"pkg.package-operator.run/boxcutter/machinery/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Prober check Kubernetes objects for certain conditions and report success or failure with failure messages.
type Prober interface {
	Probe(obj client.Object) types.ProbeResult
}

// And combines multiple Prober, only passing if all given Probers succeed.
// Messages of failing Probers will be joined together.
type And []Prober

var _ Prober = (And)(nil)

// Probe executes the probe.
func (p And) Probe(obj client.Object) types.ProbeResult {
	var allMsgs []string
	hasUnknown := false

	for _, probe := range p {
		result := probe.Probe(obj)
		switch result.Status {
		case types.ProbeStatusFalse:
			// If any probe fails, the And fails
			allMsgs = append(allMsgs, result.Messages...)
		case types.ProbeStatusUnknown:
			// Track that we have an unknown state
			hasUnknown = true
			allMsgs = append(allMsgs, result.Messages...)
		case types.ProbeStatusTrue:
			// Continue checking other probes
		}
	}

	// Determine final status:
	// - If we have any messages (from False or Unknown)
	if len(allMsgs) > 0 {
		if hasUnknown {
			return types.ProbeResult{
				Status:   types.ProbeStatusUnknown,
				Messages: allMsgs,
			}
		}
		return types.ProbeResult{
			Status:   types.ProbeStatusFalse,
			Messages: allMsgs,
		}
	}

	// All probes were True
	return types.ProbeResult{
		Status:   types.ProbeStatusTrue,
		Messages: nil,
	}
}

func toUnstructured(obj client.Object) *unstructured.Unstructured {
	unstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		panic(fmt.Sprintf("can't convert to unstructured: %v", err))
	}
	return &unstructured.Unstructured{Object: unstr}
}

func probeUnstructuredSingleMsg(
	obj client.Object,
	probe func(obj *unstructured.Unstructured) (success bool, message string),
) types.ProbeResult {
	unst := toUnstructured(obj)
	success, msg := probe(unst)
	if success {
		return types.ProbeResult{
			Status:   types.ProbeStatusTrue,
			Messages: nil,
		}
	}
	return types.ProbeResult{
		Status:   types.ProbeStatusFalse,
		Messages: []string{msg},
	}
}
