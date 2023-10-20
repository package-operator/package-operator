package probing

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Prober check Kubernetes objects for certain conditions and report success or failure with a failure message.
type Prober interface {
	Probe(obj *unstructured.Unstructured) (success bool, message string)
}

// And combines multiple Prober, only passing if all given Probers succeed.
// Messages of failing Probers will be joined with ", ", returning a single string.
type And []Prober

var _ Prober = (And)(nil)

func (p And) Probe(obj *unstructured.Unstructured) (success bool, message string) {
	var messages []string
	for _, probe := range p {
		if success, message := probe.Probe(obj); !success {
			messages = append(messages, message)
		}
	}
	if len(messages) > 0 {
		return false, strings.Join(messages, ", ")
	}
	return true, ""
}
