package probe

import (
	"fmt"
	packagesv1alpha1 "package-operator.run/package-operator/apis/core/v1alpha1"
	"testing"
)

func TestParse(t *testing.T) {
	p := packagesv1alpha1.Probe{
		FieldsEqual: &packagesv1alpha1.ProbeFieldsEqualSpec{
			FieldA: "asdf",
			FieldB: "jkl;",
		},
	}
	fmt.Println(p)
	if p.CurrentGeneration != nil {

	}
}
