package packagecontent

import (
	"context"
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
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
				},
			},
			{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
				},
			},
			{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							manifestsv1alpha1.PackageExternalObjectAnnotation: "True",
						},
					},
				},
			},
		},
	}
	files := Files{
		"xxx.yaml.gotmpl": []byte(`
apiVersion: my-group/v1alpha1
kind: MyThing
---
apiVersion: my-group/v1alpha1
kind: MyOtherThing
metadata:
  annotations:
    package-operator.run/external: "True"
`),
	}

	ctx := context.Background()
	perms, err := permissions(ctx, objectFiles, files)
	require.NoError(t, err)
	if assert.Len(t, perms.Managed, 3) {
		assert.Contains(t, perms.Managed, schema.GroupKind{Kind: "ConfigMap"})
		assert.Contains(t, perms.Managed, schema.GroupKind{Kind: "Secret"})
		assert.Contains(t, perms.Managed, schema.GroupKind{Group: "my-group", Kind: "MyThing"})
	}
	if assert.Len(t, perms.External, 2) {
		assert.Contains(t, perms.External, schema.GroupKind{Kind: "Service"})
		assert.Contains(t, perms.External, schema.GroupKind{Group: "my-group", Kind: "MyOtherThing"})
	}
}
