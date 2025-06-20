package probing

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GroupKindSelector wraps a Probe object and only executes the probe
// when the probed object is of the right Group and Kind.
// Objects not matching the slector are considered to pass the probe.
type GroupKindSelector struct {
	Prober
	schema.GroupKind
}

var _ Prober = (*GroupKindSelector)(nil)

// Probe executes the probe.
func (kp *GroupKindSelector) Probe(obj client.Object) (success bool, messages []string) {
	gk := obj.GetObjectKind().GroupVersionKind().GroupKind()
	if kp.GroupKind == gk {
		return kp.Prober.Probe(obj)
	}

	// We want to _skip_ objects, that don't match.
	// So this probe succeeds by default.
	return true, nil
}

// LabelSelector wraps a Probe object and only executes the probe
// when the probed object is matching the given label selector.
// Objects not matching the slector are considered to pass the probe.
type LabelSelector struct {
	Prober
	labels.Selector
}

var _ Prober = (*LabelSelector)(nil)

// Probe executes the probe.
func (ss *LabelSelector) Probe(obj client.Object) (success bool, messages []string) {
	if !ss.Matches(labels.Set(obj.GetLabels())) {
		// We want to _skip_ objects, that don't match.
		// So this probe succeeds by default.
		return true, nil
	}

	return ss.Prober.Probe(obj)
}
