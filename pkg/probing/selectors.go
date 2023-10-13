package probing

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupKindSelector wraps a Probe object and only executes the probe
// when the probed object is of the right Group and Kind.
// Objects not matching the slector are considered to pass the probe.
type GroupKindSelector struct {
	Prober
	schema.GroupKind
}

var _ Prober = (*GroupKindSelector)(nil)

func (kp *GroupKindSelector) Probe(obj *unstructured.Unstructured) (success bool, message string) {
	gk := obj.GetObjectKind().GroupVersionKind().GroupKind()
	if kp.GroupKind == gk {
		return kp.Prober.Probe(obj)
	}

	// We want to _skip_ objects, that don't match.
	// So this probe succeeds by default.
	return true, ""
}

// LabelSelector wraps a Probe object and only executes the probe
// when the probed object is matching the given label selector.
// Objects not matching the slector are considered to pass the probe.
type LabelSelector struct {
	Prober
	labels.Selector
}

var _ Prober = (*LabelSelector)(nil)

func (ss *LabelSelector) Probe(obj *unstructured.Unstructured) (success bool, message string) {
	if !ss.Selector.Matches(labels.Set(obj.GetLabels())) {
		// We want to _skip_ objects, that don't match.
		// So this probe succeeds by default.
		return true, ""
	}

	return ss.Prober.Probe(obj)
}
