package packagerender

import (
	"testing"

	"package-operator.run/internal/apis/manifests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"package-operator.run/apis/manifests/v1alpha1"

	"package-operator.run/internal/packages/internal/packagerender/celctx"
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
		name       string
		objects    []unstructured.Unstructured
		tmplCtx    *packagetypes.PackageRenderContext
		conditions []manifests.PackageManifestNamedCondition
		filtered   []unstructured.Unstructured
		err        string
	}{
		{
			name:       "no annotation",
			objects:    []unstructured.Unstructured{newConfigMap("a", "")},
			tmplCtx:    nil,
			conditions: nil,
			filtered:   []unstructured.Unstructured{newConfigMap("a", "")},
			err:        "",
		},
		{
			name:       "simple annotation",
			objects:    []unstructured.Unstructured{newConfigMap("a", ""), newConfigMap("b", "true && false")},
			tmplCtx:    nil,
			conditions: nil,
			filtered:   []unstructured.Unstructured{newConfigMap("a", "")},
			err:        "",
		},
		{
			name:    "condition annotation",
			objects: []unstructured.Unstructured{newConfigMap("a", ""), newConfigMap("b", "false || cond.mycondition")},
			tmplCtx: &packagetypes.PackageRenderContext{},
			conditions: []manifests.PackageManifestNamedCondition{
				{Name: "mycondition", Expression: "false"},
			},
			filtered: []unstructured.Unstructured{newConfigMap("a", "")},
			err:      "",
		},
		{
			name:    "condition annotation",
			objects: []unstructured.Unstructured{newConfigMap("a", ""), newConfigMap("b", "false || cond.mycondition")},
			tmplCtx: &packagetypes.PackageRenderContext{},
			conditions: []manifests.PackageManifestNamedCondition{
				{Name: "mycondition", Expression: "true"},
			},
			filtered: []unstructured.Unstructured{newConfigMap("a", ""), newConfigMap("b", "false || cond.mycondition")},
			err:      "",
		},
		{
			name:       "invalid expression",
			objects:    []unstructured.Unstructured{newConfigMap("a", "invalid && expression")},
			tmplCtx:    nil,
			conditions: nil,
			filtered:   nil,
			err:        string(packagetypes.ViolationReasonInvalidCELExpression),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cc, err := celctx.New(tc.conditions, tc.tmplCtx)
			require.NoError(t, err)

			filtered, err := filterWithCELAnnotation(tc.objects, &cc)
			if tc.err == "" {
				require.NoError(t, err)
				require.Equal(t, len(tc.filtered), len(filtered))
				for i := 0; i < len(filtered); i++ {
					assert.Equal(t, tc.filtered[i], filtered[i])
				}
			} else {
				require.ErrorContains(t, err, tc.err)
			}
		})
	}
}
