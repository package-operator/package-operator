package probe

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ProbeInterface interface {
	Probe(obj *unstructured.Unstructured) (success bool, message string)
}

type ProbeList []ProbeInterface

func (p ProbeList) Probe(obj *unstructured.Unstructured) (success bool, message string) {
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

// ConditionProbe checks if the object's condition is set and in a certain status.
type ConditionProbe struct {
	Type, Status string
}

var _ ProbeInterface = (*ConditionProbe)(nil)

func (cp *ConditionProbe) Probe(obj *unstructured.Unstructured) (success bool, message string) {
	defer func() {
		if success {
			return
		}
		// add probed condition type and status as context to error message.
		message = fmt.Sprintf("condition %q == %q: %s", cp.Type, cp.Status, message)
	}()

	conditions, exist, err := unstructured.
		NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !exist {
		return false, "missing .status.conditions"
	}

	for _, condI := range conditions {
		cond, ok := condI.(map[string]interface{})
		if !ok {
			// no idea what that is supposed to be
			continue
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
		} else {
			return false, "wrong status"
		}
	}
	return false, "not reported"
}

// FieldsEqualProbe checks if the values of the fields under the given json paths are equal.
type FieldsEqualProbe struct {
	FieldA, FieldB string
}

var _ ProbeInterface = (*FieldsEqualProbe)(nil)

func (fe *FieldsEqualProbe) Probe(obj *unstructured.Unstructured) (success bool, message string) {
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

	return equality.Semantic.DeepEqual(fieldAVal, fieldBVal), fmt.Sprintf("%s != %s", fieldAVal, fieldBVal)
}

// CurrentGenerationProbe ensures that the object's status is up-to-date with the object's generation.
// Requires the probed object to have a .status.observedGeneration property.
type CurrentGenerationProbe struct {
	ProbeInterface
}

var _ ProbeInterface = (*CurrentGenerationProbe)(nil)

func (cg *CurrentGenerationProbe) Probe(obj *unstructured.Unstructured) (success bool, message string) {
	if observedGeneration, ok, err := unstructured.NestedInt64(
		obj.Object, "status", "observedGeneration",
	); err == nil && ok && observedGeneration != obj.GetGeneration() {
		return false, ".status outdated"
	}
	return true, "" //cg.ProbeInterface.Probe(obj)
}
