package packagerender

import (
	"testing"

	"package-operator.run/internal/apis/manifests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"package-operator.run/apis/manifests/v1alpha1"

	"package-operator.run/internal/packages/internal/packagetypes"
)

func newConfigMap(name, cel string) unstructured.Unstructured {
	cm := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name": "cm-" + name,
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

func TestFilterWithCELAnnotation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		objects  []unstructured.Unstructured
		tmplCtx  *packagetypes.PackageRenderContext
		snippets []manifests.PackageManifestSnippet
		filtered []unstructured.Unstructured
	}{
		{
			name:     "no annotation",
			objects:  []unstructured.Unstructured{newConfigMap("a", "")},
			tmplCtx:  nil,
			snippets: nil,
			filtered: []unstructured.Unstructured{newConfigMap("a", "")},
		},
		{
			name:     "simple annotation",
			objects:  []unstructured.Unstructured{newConfigMap("a", ""), newConfigMap("b", "true && false")},
			tmplCtx:  nil,
			snippets: nil,
			filtered: []unstructured.Unstructured{newConfigMap("a", "")},
		},
		{
			name:    "snippet annotation",
			objects: []unstructured.Unstructured{newConfigMap("a", ""), newConfigMap("b", "false || mysnippet")},
			tmplCtx: &packagetypes.PackageRenderContext{},
			snippets: []manifests.PackageManifestSnippet{
				{Name: "mysnippet", Expression: "false"},
			},
			filtered: []unstructured.Unstructured{newConfigMap("a", "")},
		},
		{
			name:    "snippet annotation",
			objects: []unstructured.Unstructured{newConfigMap("a", ""), newConfigMap("b", "false || mysnippet")},
			tmplCtx: &packagetypes.PackageRenderContext{},
			snippets: []manifests.PackageManifestSnippet{
				{Name: "mysnippet", Expression: "true"},
			},
			filtered: []unstructured.Unstructured{newConfigMap("a", ""), newConfigMap("b", "false || mysnippet")},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			filtered, err := filterWithCELAnnotation(tc.objects, tc.snippets, tc.tmplCtx)
			require.NoError(t, err)
			require.Equal(t, len(tc.filtered), len(filtered))
			for i := 0; i < len(filtered); i++ {
				assert.Equal(t, tc.filtered[i], filtered[i])
			}
		})
	}
}
