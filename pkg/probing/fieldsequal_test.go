package probing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFieldsEqual(t *testing.T) {
	t.Parallel()
	fe := &FieldsEqualProbe{
		FieldA: ".spec.fieldA",
		FieldB: ".spec.fieldB",
	}

	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		succeeds bool
		message  string
	}{
		{
			name: "simple succeeds",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"fieldA": "test",
						"fieldB": "test",
					},
				},
			},
			succeeds: true,
		},
		{
			name: "simple not equal",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"fieldA": "test",
						"fieldB": "not test",
					},
				},
			},
			succeeds: false,
			message:  `".spec.fieldA" == ".spec.fieldB": "test" != "not test"`,
		},
		{
			name: "complex succeeds",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"fieldA": map[string]interface{}{
							"fk": "fv",
						},
						"fieldB": map[string]interface{}{
							"fk": "fv",
						},
					},
				},
			},
			succeeds: true,
		},
		{
			name: "simple not equal",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"fieldA": map[string]interface{}{
							"fk": "fv",
						},
						"fieldB": map[string]interface{}{
							"fk": "something else",
						},
					},
				},
			},
			succeeds: false,
			message:  `".spec.fieldA" == ".spec.fieldB": "map[fk:fv]" != "map[fk:something else]"`,
		},
		{
			name: "int not equal",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"fieldA": map[string]interface{}{
							"fk": 1.0,
						},
						"fieldB": map[string]interface{}{
							"fk": 2.0,
						},
					},
				},
			},
			succeeds: false,
			message:  `".spec.fieldA" == ".spec.fieldB": "map[fk:1]" != "map[fk:2]"`,
		},
		{
			name: "fieldA missing",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"fieldB": "test",
					},
				},
			},
			succeeds: false,
			message:  `".spec.fieldA" == ".spec.fieldB": ".spec.fieldA" missing`,
		},
		{
			name: "fieldB missing",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"fieldA": "test",
					},
				},
			},
			succeeds: false,
			message:  `".spec.fieldA" == ".spec.fieldB": ".spec.fieldB" missing`,
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			s, m := fe.Probe(test.obj)
			assert.Equal(t, test.succeeds, s)
			assert.Equal(t, test.message, m)
		})
	}
}
