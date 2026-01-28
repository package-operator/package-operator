package presets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
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
							"name":  "banana",
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

func TestDeployment(t *testing.T) {
	t.Parallel()
	scheme := &apiextensionsv1.JSONSchemaProps{}
	image := &ImageContainer{}
	out, ok, err := Parametrize(*deploy.DeepCopy(), scheme, image, ParametrizeOptions{
		Namespaces:    true,
		Replicas:      true,
		Images:        true,
		Resources:     true,
		NodeSelectors: true,

		// Need to figure out how to test with uuids present.
		// Tolerations: true,
		// Env:           true
	})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, `apiVersion: apps/v1
kind: Deployment
metadata:
  name: banana
  namespace: {{ default (index .config.namespaces "fruits") .config.namespace }}
spec:
  replicas: {{ index .config "deployments" "fruits" "banana" "replicas" }}
  template:
    spec:
      affinity: null
      containers:
      - env:
        - name: HTTP_PROXY
          value: xxx
        image: {{ index .images "banana" }}
        name: banana
        resources: {{ index .config "deployments" "fruits" "banana" "containers" "banana" "resources" | toJson }}
      nodeSelector: {{ index .config "deployments" "fruits" "banana" "nodeSelector" | toJson }}
`, string(out))
	assert.Equal(t, []manifestsv1alpha1.PackageManifestImage{
		{
			Name:  "banana",
			Image: "quay.io/package-operator/banana:latest",
		},
	}, image.List())
}

func Test_parametrizeDeploymentTolerations(t *testing.T) {
	t.Parallel()
	scheme := &apiextensionsv1.JSONSchemaProps{
		Properties: map[string]apiextensionsv1.JSONSchemaProps{},
	}
	inst, err := parametrizeDeploymentTolerations(*deploy.DeepCopy(), scheme)
	require.NoError(t, err)
	assert.Len(t, inst, 1)
}

func Test_parametrizeDeploymentContainers(t *testing.T) {
	t.Parallel()
	scheme := &apiextensionsv1.JSONSchemaProps{
		Properties: map[string]apiextensionsv1.JSONSchemaProps{},
	}
	inst, err := parametrizeDeploymentContainers(*deploy.DeepCopy(), scheme, DeploymentOptions{
		Env: true,
	})
	require.NoError(t, err)
	assert.Len(t, inst, 1)
}
