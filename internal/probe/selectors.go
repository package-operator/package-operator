package probe

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// KindSelector wraps a Probe object and only executes the probe when the probed object is of the right Group and Kind.
type KindSelector struct {
	Interface
	schema.GroupKind
}

func (kp *KindSelector) Probe(obj *unstructured.Unstructured) (success bool, message string) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if kp.Kind == gvk.Kind &&
		kp.Group == gvk.Group {
		return kp.Interface.Probe(obj)
	}

	// don't probe stuff that does not match
	return true, ""
}
