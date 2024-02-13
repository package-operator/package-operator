package packagerender

import (
	"testing"

	"github.com/google/cel-go/cel"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_evaluateCELCondition(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		expression string
		tmplCtx    *packagetypes.PackageRenderContext
		expected   bool
		err        string
	}{
		// Simple expression parsing without context
		{
			"just true",
			"true",
			nil,
			true,
			"",
		},
		{
			"simple &&",
			"true && false",
			nil,
			false,
			"",
		},
		{
			"invalid expression",
			"true && fals",
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
			false,
			newErrInvalidReturnType(cel.IntType, cel.BoolType).Error(),
		},

		// Parsing with template context
		{
			name:       "simple read from context",
			expression: ".config.banana == \"bread\"",
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
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := evaluateCELCondition(tc.expression, tc.tmplCtx)
			if tc.err == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			} else {
				require.EqualError(t, err, tc.err)
			}
		})
	}
}
