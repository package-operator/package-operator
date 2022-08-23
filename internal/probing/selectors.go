package probing

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// KindSelector wraps a Probe object and only executes the probe when the probed object is of the right Group and Kind.
type KindSelector struct {
	Prober
	schema.GroupKind
}

func (kp *KindSelector) Probe(obj *unstructured.Unstructured) (success bool, message string) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if kp.Kind == gvk.Kind &&
		kp.Group == gvk.Group {
		return kp.Prober.Probe(obj)
	}

	// We want to _skip_ objects, that don't match.
	// So this probe succeeds by default.
	return true, ""
}

type SelectorSelector struct {
	Prober
	labels.Selector
}

func (ss *SelectorSelector) Probe(obj *unstructured.Unstructured) (success bool, message string) {
	if !ss.Selector.Matches(labels.Set(obj.GetLabels())) {
		// We want to _skip_ objects, that don't match.
		// So this probe succeeds by default.
		return true, ""
	}

	return ss.Prober.Probe(obj)
}
