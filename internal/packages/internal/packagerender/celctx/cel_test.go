package celctx

import (
	"errors"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
)

func mockUnpack(unpacked map[string]any, opts []cel.EnvOption, err error) unpackContextFn {
	return func(*packagetypes.PackageRenderContext) (map[string]any, []cel.EnvOption, error) {
		return unpacked, opts, err
	}
}

func mockNewEnv(env *cel.Env, err error) newEnvFn {
	return func(...cel.EnvOption) (*cel.Env, error) {
		return env, err
	}
}

var errMock = errors.New("out of bananas")

func Test_newCelCtx(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		expression  string
		conditions  []manifests.PackageManifestNamedCondition
		tmplCtx     *packagetypes.PackageRenderContext
		unpack      unpackContextFn
		newEnv      newEnvFn
		errContains string
	}{
		{
			name:       "condition read from context",
			expression: "cond.isFoo",
			conditions: []manifests.PackageManifestNamedCondition{
				{Name: "isFoo", Expression: `.config.banana == "foo"`},
			},
			tmplCtx: &packagetypes.PackageRenderContext{
				Package:     manifests.TemplateContextPackage{},
				Config:      map[string]any{"banana": "foo"},
				Images:      nil,
				Environment: manifests.PackageEnvironment{},
			},
			unpack:      unpackContext,
			newEnv:      cel.NewEnv,
			errContains: "",
		},
		{
			name:       "invalid condition expression",
			expression: "cond.isFoo",
			conditions: []manifests.PackageManifestNamedCondition{
				{Name: "isFoo", Expression: `.config.banana "foo"`},
			},
			tmplCtx: &packagetypes.PackageRenderContext{
				Package:     manifests.TemplateContextPackage{},
				Config:      map[string]any{"banana": "foo"},
				Images:      nil,
				Environment: manifests.PackageEnvironment{},
			},
			unpack:      unpackContext,
			newEnv:      cel.NewEnv,
			errContains: ErrCELConditionEvaluation.Error(),
		},
		{
			name:       "invalid condition name",
			expression: "cond.1ustTrue",
			conditions: []manifests.PackageManifestNamedCondition{
				{Name: "1ustTrue", Expression: "true"},
			},
			tmplCtx:     nil,
			unpack:      unpackContext,
			newEnv:      cel.NewEnv,
			errContains: ErrInvalidCELConditionName.Error(),
		},
		{
			name:       "duplicate condition name",
			expression: "cond.justTrue",
			conditions: []manifests.PackageManifestNamedCondition{
				{Name: "justTrue", Expression: "true"},
				{Name: "justTrue", Expression: "false"},
			},
			tmplCtx:     nil,
			unpack:      unpackContext,
			newEnv:      cel.NewEnv,
			errContains: ErrDuplicateCELConditionName.Error(),
		},
		{
			name:       "fail unpack",
			expression: "true",
			conditions: nil,
			tmplCtx: &packagetypes.PackageRenderContext{
				Package:     manifests.TemplateContextPackage{},
				Config:      nil,
				Images:      nil,
				Environment: manifests.PackageEnvironment{},
			},
			unpack:      mockUnpack(nil, nil, errMock),
			newEnv:      cel.NewEnv,
			errContains: ErrContextUnpack.Error(),
		},
		{
			name:        "fail newEnv",
			expression:  "true",
			conditions:  nil,
			tmplCtx:     nil,
			unpack:      unpackContext,
			newEnv:      mockNewEnv(nil, errMock),
			errContains: ErrEnvCreation.Error(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := newCelCtx(tc.conditions, tc.tmplCtx, tc.unpack, tc.newEnv)
			if tc.errContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.errContains)
			}
		})
	}
}

func mockEnvProgram(fail bool) envProgramFn {
	if !fail {
		return defaultEnvProgram()
	}
	return func(*cel.Env, *cel.Ast) (cel.Program, error) {
		return nil, errMock
	}
}

func mockProgramEval(fail bool) programEvalFn {
	if !fail {
		return defaultProgramEval()
	}
	return func(cel.Program, any) (ref.Val, *cel.EvalDetails, error) {
		return nil, nil, errMock
	}
}

func Test_celCtx_evaluate(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		expression  string
		envProgram  envProgramFn
		programEval programEvalFn
		conditions  []manifests.PackageManifestNamedCondition
		tmplCtx     *packagetypes.PackageRenderContext
		expected    bool
		err         string
	}{
		// Simple expression parsing without context
		{
			name:        "just true",
			expression:  "true",
			envProgram:  defaultEnvProgram(),
			programEval: defaultProgramEval(),
			conditions:  nil,
			tmplCtx:     nil,
			expected:    true,
			err:         "",
		},
		{
			name:        "simple &&",
			expression:  "true && false",
			envProgram:  defaultEnvProgram(),
			programEval: defaultProgramEval(),
			conditions:  nil,
			tmplCtx:     nil,
			expected:    false,
			err:         "",
		},
		{
			name:        "invalid expression",
			expression:  "true && fals",
			envProgram:  defaultEnvProgram(),
			programEval: defaultProgramEval(),
			conditions:  nil,
			tmplCtx:     nil,
			expected:    false,
			err:         ErrExpressionCompilation.Error(),
		},
		{
			name:        "invalid return type",
			expression:  "2 + 3",
			envProgram:  defaultEnvProgram(),
			programEval: defaultProgramEval(),
			conditions:  nil,
			tmplCtx:     nil,
			expected:    false,
			err:         ErrInvalidReturnType.Error(),
		},

		// Parsing with template context
		{
			name:        "simple read from context",
			expression:  `config.banana == "bread"`,
			envProgram:  defaultEnvProgram(),
			programEval: defaultProgramEval(),
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
			name:        "is hypershift",
			expression:  "has(.environment.hyperShift)",
			envProgram:  defaultEnvProgram(),
			programEval: defaultProgramEval(),
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
			name:        "condition read from context",
			expression:  "cond.isFoo",
			envProgram:  defaultEnvProgram(),
			programEval: defaultProgramEval(),
			conditions: []manifests.PackageManifestNamedCondition{
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
		{
			name:        "fail program construction",
			expression:  "false",
			envProgram:  mockEnvProgram(true),
			programEval: defaultProgramEval(),
			conditions:  nil,
			tmplCtx:     nil,
			expected:    false,
			err:         ErrProgramConstruction.Error(),
		},
		{
			name:        "fail program evaluation",
			expression:  "false",
			envProgram:  defaultEnvProgram(),
			programEval: mockProgramEval(true),
			conditions:  nil,
			tmplCtx:     nil,
			expected:    false,
			err:         ErrProgramEvaluation.Error(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cc, err := New(tc.conditions, tc.tmplCtx)
			require.NoError(t, err)

			result, err := cc.evaluate(tc.expression, tc.envProgram, tc.programEval)
			if tc.err == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			} else {
				require.ErrorContains(t, err, tc.err)
			}
		})
	}
}
