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
		messages []string
	}{
		{
			name: "simple succeeds",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"spec": map[string]any{
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
				Object: map[string]any{
					"spec": map[string]any{
						"fieldA": "test",
						"fieldB": "not test",
					},
				},
			},
			succeeds: false,
			messages: []string{`".spec.fieldA" == ".spec.fieldB": "test" != "not test"`},
		},
		{
			name: "complex succeeds",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"spec": map[string]any{
						"fieldA": map[string]any{
							"fk": "fv",
						},
						"fieldB": map[string]any{
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
				Object: map[string]any{
					"spec": map[string]any{
						"fieldA": map[string]any{
							"fk": "fv",
						},
						"fieldB": map[string]any{
							"fk": "something else",
						},
					},
				},
			},
			succeeds: false,
			messages: []string{`".spec.fieldA" == ".spec.fieldB": "map[fk:fv]" != "map[fk:something else]"`},
		},
		{
			name: "int not equal",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"spec": map[string]any{
						"fieldA": map[string]any{
							"fk": 1.0,
						},
						"fieldB": map[string]any{
							"fk": 2.0,
						},
					},
				},
			},
			succeeds: false,
			messages: []string{`".spec.fieldA" == ".spec.fieldB": "map[fk:1]" != "map[fk:2]"`},
		},
		{
			name: "fieldA missing",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"spec": map[string]any{
						"fieldB": "test",
					},
				},
			},
			succeeds: false,
			messages: []string{`".spec.fieldA" == ".spec.fieldB": ".spec.fieldA" missing`},
		},
		{
			name: "fieldB missing",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"spec": map[string]any{
						"fieldA": "test",
					},
				},
			},
			succeeds: false,
			messages: []string{`".spec.fieldA" == ".spec.fieldB": ".spec.fieldB" missing`},
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := fe.Probe(test.obj)
			assert.Equal(t, test.succeeds, result.Status)
			assert.Equal(t, test.messages, result.Messages)
		})
	}
}
