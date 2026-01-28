package parametrize

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var deploy = unstructured.Unstructured{
	Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "banana",
			"namespace": "fruits",
		},
		"spec": map[string]any{
			"replicas": int64(1),
			"template": map[string]any{
				"spec": map[string]any{
					"affinity": nil,
					"containers": []any{
						map[string]any{
							"image": "quay.io/package-operator/banana:latest",
							"env": []any{
								map[string]any{
									"name":  "HTTP_PROXY",
									"value": "xxx",
								},
							},
						},
					},
				},
			},
		},
	},
}

var configMap = unstructured.Unstructured{
	Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "banana",
			"namespace": "fruits",
		},
		"data": map[string]any{
			"field1": "val1",
			"field2": "val2",
		},
	},
}

var configMapDataSlice = unstructured.Unstructured{
	Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "banana",
			"namespace": "fruits",
		},
		"data": []any{
			map[string]any{
				"field1": "val1",
			},
			map[string]any{
				"field2": "val2",
			},
		},
	},
}

func TestExecute_Deployment(t *testing.T) {
	t.Parallel()
	out, err := Execute(deploy,
		Pipeline(
			`.config.namespace`,
			"metadata.namespace",
		),
		Pipeline(
			`index .images "package-operator-manager"`,
			"spec.template.spec.containers.0.image",
		),
		Pipeline(
			`.config.replicas`,
			"spec.replicas",
		),
		Pipeline(
			`toJson .config.affinity`,
			"spec.template.spec.affinity",
		),
		Block(
			`if hasKey .config "affinity"`,
			"spec.template.spec.affinity",
		),
		Pipeline(
			`.environment.proxy.httpProxy | quote`,
			"spec.template.spec.containers.0.env.0.value",
		),
		Block(
			`if and (hasKey .environment "proxy") (hasKey .environment.proxy "httpProxy")`,
			"spec.template.spec.containers.0.env.0",
		),
	)
	require.NoError(t, err)
	assert.Equal(t, `apiVersion: apps/v1
kind: Deployment
metadata:
  name: banana
  namespace: {{ .config.namespace }}
spec:
  replicas: {{ .config.replicas }}
  template:
    spec:
      {{- if hasKey .config "affinity" }}
      affinity: {{ toJson .config.affinity }}
      {{- end }}
      containers:
      - env:
        {{- if and (hasKey .environment "proxy") (hasKey .environment.proxy "httpProxy") }}
        - name: HTTP_PROXY
          value: {{ .environment.proxy.httpProxy | quote }}
        {{- end }}
        image: {{ index .images "package-operator-manager" }}
`, string(out))
}

func TestExecute_MergeBlock(t *testing.T) {
	t.Parallel()
	t.Run("merge dicts", func(t *testing.T) {
		t.Parallel()
		out, err := Execute(configMap,
			mergeBlockWithStaticUUID(".config.data", "data"),
		)
		require.NoError(t, err)
		assert.Equal(t, `apiVersion: v1
{{- define "f1d2b2e3-bfaf-419d-ad8a-d678ca85760f" }}
data:
  field1: val1
  field2: val2
{{- end }}{{"\n"}}
{{- merge (fromYAML (include "f1d2b2e3-bfaf-419d-ad8a-d678ca85760f" .)) (.config.data)  | toYAML | indent 0 }}
kind: ConfigMap
metadata:
  name: banana
  namespace: fruits
`, string(out))
	})

	t.Run("merge slices", func(t *testing.T) {
		t.Parallel()
		out, err := Execute(configMapDataSlice,
			mergeBlockWithStaticUUID(".config.data", "data"),
		)
		require.NoError(t, err)
		//nolint:lll
		assert.Equal(t, `apiVersion: v1
{{- define "f1d2b2e3-bfaf-419d-ad8a-d678ca85760f" }}
data:
- field1: val1
- field2: val2
{{- end }}{{"\n"}}
{{- dict "data" (concat (fromYAML (include "f1d2b2e3-bfaf-419d-ad8a-d678ca85760f" .)).data (.config.data)) | toYAML | indent 0 }}
kind: ConfigMap
metadata:
  name: banana
  namespace: fruits
`, string(out))
	})
}

func mergeBlockWithStaticUUID(pipeline string, fieldPath string) *mergeBlock {
	i := MergeBlock(pipeline, fieldPath).(*mergeBlock)
	i.marker = "f1d2b2e3-bfaf-419d-ad8a-d678ca85760f"
	return i
}
