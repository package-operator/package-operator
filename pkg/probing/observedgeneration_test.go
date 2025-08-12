package probing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"pkg.package-operator.run/boxcutter/machinery/types"
)

func TestStatusObservedGeneration(t *testing.T) {
	t.Parallel()
	properMock := &proberMock{}
	og := &ObservedGenerationProbe{
		Prober: properMock,
	}

	properMock.On("Probe", mock.Anything).Return(
		types.ProbeResult{
			Status:   types.ProbeStatusTrue,
			Messages: nil,
		},
	)

	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		succeeds bool
		messages []string
	}{
		{
			name: "outdated",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"generation": int64(4),
					},
					"status": map[string]any{
						"observedGeneration": int64(2),
					},
				},
			},
			succeeds: false,
			messages: []string{".status outdated"},
		},
		{
			name: "up-to-date",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"generation": int64(4),
					},
					"status": map[string]any{
						"observedGeneration": int64(4),
					},
				},
			},
			succeeds: true,
			messages: []string{"banana"},
		},
		{
			name: "not reported",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"generation": int64(4),
					},
					"status": map[string]any{},
				},
			},
			succeeds: true,
			messages: []string{"banana"},
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := og.Probe(test.obj)
			assert.Equal(t, test.succeeds, result.Status)
			assert.Equal(t, test.messages, result.Messages)
		})
	}
}
