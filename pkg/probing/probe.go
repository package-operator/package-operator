package probing

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Prober check Kubernetes objects for certain conditions and report success or failure with failure messages.
type Prober interface {
	Probe(obj client.Object) (success bool, messages []string)
}

// And combines multiple Prober, only passing if all given Probers succeed.
// Messages of failing Probers will be joined together.
type And []Prober

var _ Prober = (And)(nil)

// Probe executes the probe.
func (p And) Probe(obj client.Object) (success bool, messages []string) {
	var allMsgs []string
	for _, probe := range p {
		if success, msgs := probe.Probe(obj); !success {
			allMsgs = append(allMsgs, msgs...)
		}
	}
	if len(allMsgs) > 0 {
		return false, allMsgs
	}
	return true, nil
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
) (success bool, messages []string) {
	unst := toUnstructured(obj)
	success, msg := probe(unst)
	if success {
		return success, nil
	}
	return success, []string{msg}
}
