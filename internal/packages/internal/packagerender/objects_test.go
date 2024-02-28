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

func TestFilterWithCELAnnotation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		objects  []unstructured.Unstructured
		tmplCtx  *packagetypes.PackageRenderContext
		macros   map[string]string
		filtered []unstructured.Unstructured
	}{
		{
			name:     "no annotation",
			objects:  []unstructured.Unstructured{newConfigMap("")},
			tmplCtx:  nil,
			macros:   nil,
			filtered: []unstructured.Unstructured{newConfigMap("")},
		},
		{
			name:     "simple annotation",
			objects:  []unstructured.Unstructured{newConfigMap(""), newConfigMap("true && false")},
			tmplCtx:  nil,
			macros:   nil,
			filtered: []unstructured.Unstructured{newConfigMap("")},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			filtered, err := filterWithCELAnnotation(tc.objects, tc.tmplCtx, tc.macros)
			require.NoError(t, err)
			require.Equal(t, len(tc.filtered), len(filtered))
			for i := 0; i < len(filtered); i++ {
				assert.Equal(t, tc.filtered[i], filtered[i])
			}
		})
	}
}

func TestReplaceMacros(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		expression string
		macros     map[string]string
		result     string
	}{
		{
			name:       "no macros",
			expression: "true && false",
			macros:     nil,
			result:     "true && false",
		},
		{
			name:       "simple replace",
			expression: "justTrue || false",
			macros:     map[string]string{"justTrue": "true"},
			result:     "true || false",
		},
		{
			name:       "multiple replace",
			expression: "(justTrue || notTrue) && notTrue",
			macros: map[string]string{
				"justTrue": "true",
				"notTrue":  "false",
			},
			result: "(true || false) && false",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.result, replaceMacros(tc.expression, tc.macros))
		})
	}
}

func TestEvaluateCELMacros(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		macros  []manifests.PackageManifestCelMacro
		tmplCtx *packagetypes.PackageRenderContext
		result  map[string]string
		err     error
	}{
		{
			name:    "no macros",
			macros:  nil,
			tmplCtx: nil,
			result:  map[string]string{},
			err:     nil,
		},
		{
			name: "invalid name",
			macros: []manifests.PackageManifestCelMacro{
				{
					Name:       "1ustTrue&",
					Expression: "true",
				},
			},
			tmplCtx: nil,
			result:  nil,
			err:     ErrInvalidCELMacroName,
		},
		{
			name: "duplicate macro names",
			macros: []manifests.PackageManifestCelMacro{
				{
					Name:       "justTrue",
					Expression: "true",
				},
				{
					Name:       "notFalse",
					Expression: "true",
				},
				{
					Name:       "justTrue",
					Expression: "false",
				},
			},
			tmplCtx: nil,
			result:  nil,
			err:     ErrDuplicateCELMacroName,
		},
		{
			name: "simple eval",
			macros: []manifests.PackageManifestCelMacro{
				{
					Name:       "justTrue",
					Expression: "true",
				},
				{
					Name:       "notFalse",
					Expression: "(false || true) && (!false)",
				},
				{
					Name:       "justFalse",
					Expression: "(true && false) || false",
				},
			},
			tmplCtx: nil,
			result: map[string]string{
				"justTrue":  "true",
				"notFalse":  "true",
				"justFalse": "false",
			},
			err: nil,
		},
		{
			name: "eval with context",
			macros: []manifests.PackageManifestCelMacro{
				{
					Name:       "justTrue",
					Expression: "has(.images.not_there) || (has(.config.banana) && .config.banana == \"bread\")",
				},
				{
					Name:       "justFalse",
					Expression: "has(.environment.hyperShift)",
				},
			},
			tmplCtx: &packagetypes.PackageRenderContext{
				Package:     manifests.TemplateContextPackage{},
				Config:      map[string]any{"banana": "bread"},
				Images:      nil,
				Environment: manifests.PackageEnvironment{},
			},
			result: map[string]string{
				"justTrue":  "true",
				"justFalse": "false",
			},
			err: nil,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			eval, err := evaluateCELMacros(tc.macros, tc.tmplCtx)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.result, eval)
			}
		})
	}
}
