package packagecontent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func Test_permissions(t *testing.T) {
	t.Parallel()

	objectFiles := map[string][]unstructured.Unstructured{
		"test.yaml": {
			{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
				},
			},
			{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
				},
			},
			{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Service",
					"metadata": map[string]any{
						"annotations": map[string]any{
							manifestsv1alpha1.PackageExternalObjectAnnotation: "True",
						},
					},
				},
			},
		},
	}

	meta, err := metadata(objectFiles)
	require.NoError(t, err)
	if assert.Len(t, meta.ManagedObjectTypes, 3) {
		assert.Contains(t, meta.ManagedObjectTypes, schema.GroupKind{Kind: "ConfigMap"})
		assert.Contains(t, meta.ManagedObjectTypes, schema.GroupKind{Kind: "Secret"})
		assert.Contains(t, meta.ManagedObjectTypes, schema.GroupKind{Group: "my-group", Kind: "MyThing"})
	}
	if assert.Len(t, meta.ExternalObjectTypes, 2) {
		assert.Contains(t, meta.ExternalObjectTypes, schema.GroupKind{Kind: "Service"})
		assert.Contains(t, meta.ExternalObjectTypes, schema.GroupKind{Group: "my-group", Kind: "MyOtherThing"})
	}
}
