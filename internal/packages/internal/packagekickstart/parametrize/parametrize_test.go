package parametrize

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var deploy = unstructured.Unstructured{
	Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "banana",
			"namespace": "fruits",
		},
		"spec": map[string]interface{}{
			"replicas": int64(1),
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"affinity": nil,
					"containers": []interface{}{
						map[string]interface{}{
							"image": "quay.io/package-operator/banana:latest",
							"env": []interface{}{
								map[string]interface{}{
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

func TestExecute(t *testing.T) {
	out, err := Execute(deploy,
		Expression(
			`.config.namespace`,
			"metadata.namespace",
		),
		Expression(
			`index .images "package-operator-manager"`,
			"spec.template.spec.containers.0.image",
		),
		Expression(
			`.config.replicas`,
			"spec.replicas",
		),
		Expression(
			`toJson .config.affinity`,
			"spec.template.spec.affinity",
		),
		Block(
			`if hasKey .config "affinity"`,
			"spec.template.spec.affinity",
		),
		Expression(
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

func TestParametrize(t *testing.T) {
	scheme := &apiextensionsv1.JSONSchemaProps{}
	out, ok, err := Parametrize(deploy, scheme, []string{"replicas"})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, `apiVersion: apps/v1
kind: Deployment
metadata:
  name: banana
  namespace: fruits
spec:
  replicas: {{ .config.deployments.fruits.banana.replicas }}
  template:
    spec:
      affinity: null
      containers:
      - env:
        - name: HTTP_PROXY
          value: xxx
        image: quay.io/package-operator/banana:latest
`, string(out))
}
