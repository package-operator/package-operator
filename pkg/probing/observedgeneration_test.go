package probing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestStatusObservedGeneration(t *testing.T) {
	t.Parallel()
	properMock := &proberMock{}
	og := &ObservedGenerationProbe{
		Prober: properMock,
	}

	properMock.On("Probe", mock.Anything).Return(true, "banana")

	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		succeeds bool
		message  string
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
			message:  ".status outdated",
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
			message:  "banana",
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
			message:  "banana",
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			s, m := og.Probe(test.obj)
			assert.Equal(t, test.succeeds, s)
			assert.Equal(t, test.message, m)
		})
	}
}
