package presets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	configMap = unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "banana",
				"namespace": "fruits",
			},
		},
	}

	secret = unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":      "banana",
				"namespace": "fruits",
			},
		},
	}

	clusterRoleBinding = unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRoleBinding",
			"metadata": map[string]any{
				"name": "banana",
			},
			"roleRef": map[string]any{
				"kind": "ClusterRole",
				"name": "bananas",
			},
			"subjects": []any{
				map[string]any{
					"kind": "User",
					"name": "hans",
				},
				map[string]any{
					"kind":      "ServiceAccount",
					"name":      "banana",
					"namespace": "fruits",
				},
			},
		},
	}
)

func TestGeneric(t *testing.T) {
	t.Parallel()
	out, ok, err := Generic(configMap, GenericOptions{})
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, out)
}

func TestGeneric_Namespace(t *testing.T) {
	t.Parallel()
	t.Run("ConfigMap", func(t *testing.T) {
		t.Parallel()
		scheme := &v1.JSONSchemaProps{
			Type:       "object",
			Properties: map[string]v1.JSONSchemaProps{},
		}
		ic := &ImageContainer{}
		out, ok, err := Parametrize(configMap, scheme, ic, ParametrizeOptions{
			Namespaces: true,
		})
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, `apiVersion: v1
kind: ConfigMap
metadata:
  name: banana
  namespace: {{ default (index .config.namespaces "fruits") .config.namespace }}
`, string(out))
	})

	t.Run("Secret", func(t *testing.T) {
		t.Parallel()
		scheme := &v1.JSONSchemaProps{
			Type:       "object",
			Properties: map[string]v1.JSONSchemaProps{},
		}
		ic := &ImageContainer{}
		out, ok, err := Parametrize(secret, scheme, ic, ParametrizeOptions{
			Namespaces: true,
		})
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, `apiVersion: v1
kind: Secret
metadata:
  name: banana
  namespace: {{ default (index .config.namespaces "fruits") .config.namespace }}
`, string(out))
	})

	t.Run("ClusterRoleBinding", func(t *testing.T) {
		t.Parallel()
		// templates subject namespaces
		scheme := &v1.JSONSchemaProps{
			Type:       "object",
			Properties: map[string]v1.JSONSchemaProps{},
		}
		ic := &ImageContainer{}
		out, ok, err := Parametrize(clusterRoleBinding, scheme, ic, ParametrizeOptions{
			Namespaces: true,
		})
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: banana
roleRef:
  kind: ClusterRole
  name: bananas
subjects:
- kind: User
  name: hans
- kind: ServiceAccount
  name: banana
  namespace: {{ default (index .config.namespaces "fruits") .config.namespace }}
`, string(out))
	})
}
