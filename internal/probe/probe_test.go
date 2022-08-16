package probe

import (
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"testing"
)

var test = unstructured.Unstructured{
	Object: map[string]interface{}{
		"kind":       "test_kind",
		"apiVersion": "test_version",
		"metadata": map[string]interface{}{
			"name":      "test_name",
			"namespace": "test_namespace",
			"status": map[string]interface{}{ // TODO: is this right? unstructured.SetOwnerReferences sets them as []interface{}
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Available",
						"status": "False",
					},
				},
				"observedGeneration": int64(1),
			},
		},
	},
}

var test2 = unstructured.Unstructured{
	Object: map[string]interface{}{
		"kind":       "test_kind",
		"apiVersion": "test_version",
		"metadata": map[string]interface{}{
			"name":         "test",
			"namespace":    "test",
			"generateName": "test_generateName",
		},
		"status": map[string]interface{}{ // TODO: is this right? unstructured.SetOwnerReferences sets them as []interface{}
			"conditions": []interface{}{
				map[string]interface{}{
					"type":   "Available",
					"status": "True",
				},
			},
			"observedGeneration": int64(1),
		},
	},
}

var test3 = unstructured.Unstructured{
	Object: map[string]interface{}{
		"kind":       "test_kind",
		"apiVersion": "test_version",
		"metadata": map[string]interface{}{
			"name":      "test",
			"namespace": "test",
			"status": map[string]interface{}{ // TODO: is this right? unstructured.SetOwnerReferences sets them as []interface{}
				"conditions": []interface{}{
					map[string]interface{}{
						"type":               "Available",
						"status":             "True",
						"observedGeneration": int64(1),
					},
				},
				"observedGeneration": int64(1),
			},
		},
	},
}

func TestProbe(t *testing.T) {
	tests := []struct {
		name                  string
		obj                   *unstructured.Unstructured
		passFieldEqual        bool
		passCondition         bool
		passCurrentGeneration bool
	}{
		{
			name:                  "Fields unequal, condition wrong, up to date generation",
			obj:                   &test,
			passFieldEqual:        false,
			passCondition:         false,
			passCurrentGeneration: true,
		},
		{
			name:                  "Fields equal, condition correct, up to date generation",
			obj:                   &test2,
			passFieldEqual:        true,
			passCondition:         true,
			passCurrentGeneration: true,
		},
		{
			name:                  "Fields equal, condition correct, out dated generation",
			obj:                   &test3,
			passFieldEqual:        true,
			passCondition:         false,
			passCurrentGeneration: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cp := ConditionProbe{
				Type:   "Available",
				Status: "True",
			}
			success, _ := cp.Probe(test.obj)
			assert.Equal(t, test.passCondition, success, "condition probe failed")

			fep := FieldsEqualProbe{
				FieldA: "metadata.name",
				FieldB: "metadata.namespace",
			}
			success, _ = fep.Probe(test.obj)
			assert.Equal(t, test.passFieldEqual, success, "fields equal probe failed")

			cgp := CurrentGenerationProbe{}
			success, _ = cgp.Probe(test.obj)
			assert.Equal(t, test.passCurrentGeneration, success, "current generation probe failed")
		})
	}
}
