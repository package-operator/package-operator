package packagerender

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"package-operator.run/apis/manifests/v1alpha1"

	"package-operator.run/internal/packages/internal/packagetypes"
)

func newConfigMap(cel string) unstructured.Unstructured {
	cm := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name": "cm",
			},
			"data": map[string]any{
				"banana": "bread",
			},
		},
	}

	if cel != "" {
		cm.SetAnnotations(map[string]string{v1alpha1.PackageCELConditionAnnotation: cel})
	}

	return cm
}

func Test_filterWithCELAnnotation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		objects  []unstructured.Unstructured
		tmplCtx  *packagetypes.PackageRenderContext
		filtered []unstructured.Unstructured
	}{
		{
			name:     "no annotation",
			objects:  []unstructured.Unstructured{newConfigMap("")},
			tmplCtx:  nil,
			filtered: []unstructured.Unstructured{newConfigMap("")},
		},
		{
			name:     "simple annotation",
			objects:  []unstructured.Unstructured{newConfigMap(""), newConfigMap("true && false")},
			tmplCtx:  nil,
			filtered: []unstructured.Unstructured{newConfigMap("")},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			filtered, err := filterWithCELAnnotation(tc.objects, tc.tmplCtx)
			require.NoError(t, err)
			require.Equal(t, len(tc.filtered), len(filtered))
			for i := 0; i < len(filtered); i++ {
				assert.Equal(t, tc.filtered[i], filtered[i])
			}
		})
	}
}
