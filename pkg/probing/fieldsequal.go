package probing

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"pkg.package-operator.run/boxcutter/machinery/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FieldsEqualProbe checks if the values of the fields under the given json paths are equal.
type FieldsEqualProbe struct {
	FieldA, FieldB string
}

var _ Prober = (*FieldsEqualProbe)(nil)

// Probe executes the probe.
func (fe *FieldsEqualProbe) Probe(obj client.Object) types.ProbeResult {
	return probeUnstructuredSingleMsg(obj, fe.probe)
}

func (fe *FieldsEqualProbe) probe(obj *unstructured.Unstructured) (success bool, message string) {
	fieldAPath := strings.Split(strings.Trim(fe.FieldA, "."), ".")
	fieldBPath := strings.Split(strings.Trim(fe.FieldB, "."), ".")

	defer func() {
		if success {
			return
		}
		// add probed field paths as context to error message.
		message = fmt.Sprintf(`"%v" == "%v": %s`, fe.FieldA, fe.FieldB, message)
	}()

	fieldAVal, ok, err := unstructured.NestedFieldCopy(obj.Object, fieldAPath...)
	if err != nil || !ok {
		return false, fmt.Sprintf(`"%v" missing`, fe.FieldA)
	}
	fieldBVal, ok, err := unstructured.NestedFieldCopy(obj.Object, fieldBPath...)
	if err != nil || !ok {
		return false, fmt.Sprintf(`"%v" missing`, fe.FieldB)
	}

	if !equality.Semantic.DeepEqual(fieldAVal, fieldBVal) {
		return false, fmt.Sprintf(`"%v" != "%v"`, fieldAVal, fieldBVal)
	}
	return true, ""
}
