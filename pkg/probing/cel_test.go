package probing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_NewCELProbe(t *testing.T) {
	t.Parallel()

	_, err := NewCELProbe(`self.test`, "")
	require.ErrorIs(t, err, ErrCELInvalidEvaluationType)
}

func Test_celProbe(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		rule, message string
		obj           *unstructured.Unstructured

		success bool
	}{
		{
			name:    "simple success",
			rule:    `self.metadata.name == "hans"`,
			message: "aaaaaah!",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name": "hans",
					},
				},
			},
			success: true,
		},
		{
			name:    "simple failure",
			rule:    `self.metadata.name == "hans"`,
			message: "aaaaaah!",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name": "nothans",
					},
				},
			},
			success: false,
		},
		{
			name:    "OpenShift Route success simple",
			rule:    `self.status.ingress.all(i, i.conditions.all(c, c.type == "Ready" && c.status == "True"))`,
			message: "aaaaaah!",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{
						"test": []any{"1", "2", "3"},
						"ingress": []any{
							map[string]any{
								"host": "hostname.xxx.xxx",
								"conditions": []any{
									map[string]any{
										"type":   "Ready",
										"status": "True",
									},
								},
							},
						},
					},
				},
			},
			success: true,
		},
		{
			name:    "OpenShift Route failure",
			rule:    `self.status.ingress.all(i, i.conditions.all(c, c.type == "Ready" && c.status == "True"))`,
			message: "aaaaaah!",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"status": map[string]any{
						"test": []any{"1", "2", "3"},
						"ingress": []any{
							map[string]any{
								"host": "hostname.xxx.xxx",
								"conditions": []any{
									map[string]any{
										"type":   "Ready",
										"status": "True",
									},
								},
							},
							map[string]any{
								"host": "otherhost.xxx.xxx",
								"conditions": []any{
									map[string]any{
										"type":   "Ready",
										"status": "False",
									},
								},
							},
						},
					},
				},
			},
			success: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			p, err := NewCELProbe(test.rule, test.message)
			require.NoError(t, err)

			success, outMsg := p.Probe(test.obj)
			assert.Equal(t, test.success, success)
			assert.Equal(t, test.message, outMsg)
		})
	}
}
