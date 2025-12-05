package probing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCondition(t *testing.T) {
	t.Parallel()
	c := &ConditionProbe{
		Type:   "Available",
		Status: "False",
	}

	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		succeeds bool
		messages []string
	}{
		{
			name: "succeeds",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"type":               "Banana",
								"status":             "True",
								"observedGeneration": int64(1), // up to date
							},
							map[string]any{
								"type":               "Available",
								"status":             "False",
								"observedGeneration": int64(1), // up to date
							},
						},
					},
					"metadata": map[string]any{
						"generation": int64(1),
					},
				},
			},
			succeeds: true,
		},
		{
			name: "outdated",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"type":               "Available",
								"status":             "False",
								"observedGeneration": int64(1), // up to date
							},
						},
					},
					"metadata": map[string]any{
						"generation": int64(42),
					},
				},
			},
			succeeds: false,
			messages: []string{`condition "Available" == "False": outdated`},
		},
		{
			name: "wrong status",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"type":               "Available",
								"status":             "Unknown",
								"observedGeneration": int64(1), // up to date
							},
						},
					},
					"metadata": map[string]any{
						"generation": int64(1),
					},
				},
			},
			succeeds: false,
			messages: []string{`condition "Available" == "False": wrong status`},
		},
		{
			name: "not reported",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"type":               "Banana",
								"status":             "True",
								"observedGeneration": int64(1), // up to date
							},
						},
					},
					"metadata": map[string]any{
						"generation": int64(1),
					},
				},
			},
			succeeds: false,
			messages: []string{`condition "Available" == "False": not reported`},
		},
		{
			name: "malformed condition type int",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{
						"conditions": []any{
							42, 56,
						},
					},
					"metadata": map[string]any{
						"generation": int64(1),
					},
				},
			},
			succeeds: false,
			messages: []string{`condition "Available" == "False": malformed`},
		},
		{
			name: "malformed condition type string",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{
						"conditions": []any{
							"42", "56",
						},
					},
					"metadata": map[string]any{
						"generation": int64(1),
					},
				},
			},
			succeeds: false,
			messages: []string{`condition "Available" == "False": malformed`},
		},
		{
			name: "malformed conditions array",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{
						"conditions": 42,
					},
					"metadata": map[string]any{
						"generation": int64(1),
					},
				},
			},
			succeeds: false,
			messages: []string{`condition "Available" == "False": malformed`},
		},
		{
			name: "missing conditions",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{},
					"metadata": map[string]any{
						"generation": int64(1),
					},
				},
			},
			succeeds: false,
			messages: []string{`condition "Available" == "False": missing .status.conditions`},
		},
		{
			name: "missing status",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"generation": int64(1),
					},
				},
			},
			succeeds: false,
			messages: []string{`condition "Available" == "False": missing .status.conditions`},
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := c.Probe(test.obj)
			assert.Equal(t, test.succeeds, result.Status)
			assert.Equal(t, test.messages, result.Messages)
		})
	}
}
