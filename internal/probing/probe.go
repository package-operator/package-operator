package probing

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Prober interface {
	Probe(obj *unstructured.Unstructured) (success bool, message string)
}

type list []Prober

var _ Prober = (list)(nil)

func (p list) Probe(obj *unstructured.Unstructured) (success bool, message string) {
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

// condition checks if the object's condition is set and in a certain status.
type condition struct {
	Type, Status string
}

var _ Prober = (*condition)(nil)

func (cp *condition) Probe(obj *unstructured.Unstructured) (success bool, message string) {
	defer func() {
		if success {
			return
		}
		// add probed condition type and status as context to error message.
		message = fmt.Sprintf("condition %q == %q: %s", cp.Type, cp.Status, message)
	}()

	rawConditions, exist, err := unstructured.NestedFieldNoCopy(
		obj.Object, "status", "conditions")
	conditions, ok := rawConditions.([]interface{})
	if err != nil || !exist {
		return false, "missing .status.conditions"
	}
	if !ok {
		return false, "malformed"
	}

	for _, condI := range conditions {
		cond, ok := condI.(map[string]interface{})
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

// FieldsEqual checks if the values of the fields under the given json paths are equal.
type fieldsEqual struct {
	FieldA, FieldB string
}

var _ Prober = (*fieldsEqual)(nil)

func (fe *fieldsEqual) Probe(obj *unstructured.Unstructured) (success bool, message string) {
	fieldAPath := strings.Split(strings.Trim(fe.FieldA, "."), ".")
	fieldBPath := strings.Split(strings.Trim(fe.FieldB, "."), ".")

	defer func() {
		if success {
			return
		}
		// add probed field paths as context to error message.
		message = fmt.Sprintf("%q == %q: %s", fe.FieldA, fe.FieldB, message)
	}()

	fieldAVal, ok, err := unstructured.NestedFieldCopy(obj.Object, fieldAPath...)
	if err != nil || !ok {
		return false, fmt.Sprintf("%q missing", fe.FieldA)
	}
	fieldBVal, ok, err := unstructured.NestedFieldCopy(obj.Object, fieldBPath...)
	if err != nil || !ok {
		return false, fmt.Sprintf("%q missing", fe.FieldB)
	}

	if !equality.Semantic.DeepEqual(fieldAVal, fieldBVal) {
		return false, fmt.Sprintf("%q != %q", fieldAVal, fieldBVal)
	}
	return true, ""
}

// StatusObservedGeneration wraps the given Prober and ensures that .status.observedGeneration is qual to .metadata.generation,
// before running the given probe. If the probed object does not contain the .status.observedGeneration field,
// the given prober is executed directly.
type statusObservedGeneration struct {
	Prober
}

var _ Prober = (*statusObservedGeneration)(nil)

func (cg *statusObservedGeneration) Probe(obj *unstructured.Unstructured) (success bool, message string) {
	if observedGeneration, ok, err := unstructured.NestedInt64(
		obj.Object, "status", "observedGeneration",
	); err == nil && ok && observedGeneration != obj.GetGeneration() {
		return false, ".status outdated"
	}
	return cg.Prober.Probe(obj)
}
