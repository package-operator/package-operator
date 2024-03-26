package celctx

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
)

func Test_newCelCtx(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		expression  string
		macros      []manifests.PackageManifestCelMacro
		tmplCtx     *packagetypes.PackageRenderContext
		errContains string
	}{
		{
			name:       "macro read from context",
			expression: "isFoo",
			macros: []manifests.PackageManifestCelMacro{
				{Name: "isFoo", Expression: `.config.banana == "foo"`},
			},
			tmplCtx: &packagetypes.PackageRenderContext{
				Package:     manifests.TemplateContextPackage{},
				Config:      map[string]any{"banana": "foo"},
				Images:      nil,
				Environment: manifests.PackageEnvironment{},
			},
			errContains: "",
		},
		{
			name:       "invalid macro expression",
			expression: "isFoo",
			macros: []manifests.PackageManifestCelMacro{
				{Name: "isFoo", Expression: `.config.banana "foo"`},
			},
			tmplCtx: &packagetypes.PackageRenderContext{
				Package:     manifests.TemplateContextPackage{},
				Config:      map[string]any{"banana": "foo"},
				Images:      nil,
				Environment: manifests.PackageEnvironment{},
			},
			errContains: `CEL macro evaluation failed`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := New(tc.macros, tc.tmplCtx)
			if tc.errContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.errContains)
			}
		})
	}
}

func Test_celCtx_Evaluate(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		expression string
		macros     []manifests.PackageManifestCelMacro
		tmplCtx    *packagetypes.PackageRenderContext
		expected   bool
		err        string
	}{
		// Simple expression parsing without context
		{
			"just true",
			"true",
			nil,
			nil,
			true,
			"",
		},
		{
			"simple &&",
			"true && false",
			nil,
			nil,
			false,
			"",
		},
		{
			"invalid expression",
			"true && fals",
			nil,
			nil,
			false,
			"compile error: ERROR: <input>:1:9: undeclared reference to 'fals' (in container '')\n" +
				" | true && fals\n" +
				" | ........^",
		},
		{
			"invalid return type",
			"2 + 3",
			nil,
			nil,
			false,
			newErrInvalidReturnType(cel.IntType, cel.BoolType).Error(),
		},

		// Parsing with template context
		{
			name:       "simple read from context",
			expression: `config.banana == "bread"`,
			tmplCtx: &packagetypes.PackageRenderContext{
				Package:     manifests.TemplateContextPackage{},
				Config:      map[string]any{"banana": "bread"},
				Images:      nil,
				Environment: manifests.PackageEnvironment{},
			},
			expected: true,
			err:      "",
		},
		{
			name:       "is hypershift",
			expression: "has(.environment.hyperShift)",
			tmplCtx: &packagetypes.PackageRenderContext{
				Package: manifests.TemplateContextPackage{},
				Config:  nil,
				Images:  nil,
				Environment: manifests.PackageEnvironment{
					Kubernetes: manifests.PackageEnvironmentKubernetes{},
					OpenShift:  nil,
					Proxy:      nil,
					HyperShift: &manifests.PackageEnvironmentHyperShift{
						HostedCluster: &manifests.PackageEnvironmentHyperShiftHostedCluster{
							TemplateContextObjectMeta: manifests.TemplateContextObjectMeta{
								Name:      "banana",
								Namespace: "bread",
							},
							HostedClusterNamespace: "pancake",
						},
					},
				},
			},
			expected: true,
			err:      "",
		},
		{
			name:       "macro read from context",
			expression: "isFoo",
			macros: []manifests.PackageManifestCelMacro{
				{Name: "isFoo", Expression: `.config.banana == "foo"`},
			},
			tmplCtx: &packagetypes.PackageRenderContext{
				Package:     manifests.TemplateContextPackage{},
				Config:      map[string]any{"banana": "foo"},
				Images:      nil,
				Environment: manifests.PackageEnvironment{},
			},
			expected: true,
			err:      "",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cc, err := New(tc.macros, tc.tmplCtx)
			require.NoError(t, err)

			result, err := cc.Evaluate(tc.expression)
			if tc.err == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			} else {
				require.EqualError(t, err, tc.err)
			}
		})
	}
}
