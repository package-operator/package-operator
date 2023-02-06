package packagecontent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func TestParseConditionMap(t *testing.T) {
	tests := []struct {
		name             string
		object           *unstructured.Unstructured
		expectedMappings []corev1alpha1.ConditionMapping
	}{
		{
			name: "success",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							manifestsv1alpha1.PackageConditionMapAnnotation: "Available => my-prefix/Available\nSomethingElse => my-prefix/SomethingElse",
						},
					},
				},
			},
			expectedMappings: []corev1alpha1.ConditionMapping{
				{
					SourceType:      "Available",
					DestinationType: "my-prefix/Available",
				},
				{
					SourceType:      "SomethingElse",
					DestinationType: "my-prefix/SomethingElse",
				},
			},
		},
		{
			name:   "no annotation",
			object: &unstructured.Unstructured{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mappings, err := ParseConditionMapAnnotation(test.object)
			require.NoError(t, err)

			assert.Equal(t, test.expectedMappings, mappings)
		})
	}
}

func TestParseConditionMap_error(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		err        string
	}{
		{
			name:       "missing destinationType",
			annotation: "Available =>",
			err:        "destinationType can't be empty in line 1",
		},
		{
			name:       "missing sourceType",
			annotation: "=> bla",
			err:        "sourceType can't be empty in line 1",
		},
		{
			name:       "nothing",
			annotation: "xxxx",
			err:        "expected 2 part mapping got 1 in line 1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							manifestsv1alpha1.PackageConditionMapAnnotation: test.annotation,
						},
					},
				},
			}

			_, err := ParseConditionMapAnnotation(obj)
			require.EqualError(t, err, test.err)
		})
	}

}
