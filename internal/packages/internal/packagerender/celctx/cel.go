package celctx

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
)

var (
	errContextUnpack             = errors.New("context unpacking error")
	errEnvCreation               = errors.New("CEL environment creation error")
	errExpressionCompilation     = errors.New("CEL expression compilation error")
	errProgramConstruction       = errors.New("program construction error")
	errProgramEvaluation         = errors.New("program evaluation error")
	errInvalidReturnType         = errors.New("invalid return type")
	errDuplicateCELConditionName = errors.New("duplicate CEL condition name")
	errCELConditionEvaluation    = errors.New("CEL condition evaluation failed")
	errInvalidCELConditionName   = errors.New("invalid CEL condition name")

	conditionNameRegexp = regexp.MustCompile("^[_a-zA-Z][_a-zA-Z0-9]*$")
)

type (
	unpackContextFn func(*packagetypes.PackageRenderContext) (map[string]any, []cel.EnvOption, error)
	newEnvFn        func(...cel.EnvOption) (*cel.Env, error)
	envProgramFn    func(*cel.Env, *cel.Ast) (cel.Program, error)
	programEvalFn   func(cel.Program, any) (ref.Val, *cel.EvalDetails, error)
)

// CelCtx contains a cel environment that is prepared with a tmplCtx and pre-evaluated conditions.
type CelCtx struct {
	env    *cel.Env
	ctxMap map[string]any
}

// New pre-evaluates the given named conditions against tmplCtx
// and exposes both tmplCtx + named condition results to cel programs.
func New(conditions []manifests.PackageManifestNamedCondition,
	tmplCtx *packagetypes.PackageRenderContext,
) (CelCtx, error) {
	return newCelCtx(conditions, tmplCtx, unpackContext, cel.NewEnv)
}

func newCelCtx(conditions []manifests.PackageManifestNamedCondition,
	tmplCtx *packagetypes.PackageRenderContext,
	unpack unpackContextFn,
	newEnv newEnvFn,
) (CelCtx, error) {
	ctxMap, opts, err := unpack(tmplCtx)
	if err != nil {
		return CelCtx{}, fmt.Errorf("%w: %w", errContextUnpack, err)
	}

	// create CEL environment with context
	env, err := newEnv(opts...)
	if err != nil {
		return CelCtx{}, fmt.Errorf("%w: %w", errEnvCreation, err)
	}

	cc := CelCtx{
		env:    env,
		ctxMap: ctxMap,
	}

	conditionsMap := map[string]bool{}
	for _, m := range conditions {
		// make sure condition name is allowed
		if !conditionNameRegexp.MatchString(m.Name) {
			return CelCtx{}, fmt.Errorf("%w: '%s'", errInvalidCELConditionName, m.Name)
		}

		// make sure name is unique and does not shadow a key in conditionsMap
		if _, ok := conditionsMap[m.Name]; ok {
			return CelCtx{}, fmt.Errorf("%w: '%s'", errDuplicateCELConditionName, m.Name)
		}

		result, err := cc.Evaluate(m.Expression)
		if err != nil {
			return CelCtx{}, fmt.Errorf("%w: '%s': %w", errCELConditionEvaluation, m.Name, err)
		}

		// store evaluation result in context
		conditionsMap[m.Name] = result
	}

	ctxMap["cond"] = conditionsMap
	opts = append(opts, cel.Variable("cond", cel.MapType(cel.StringType, cel.BoolType)))

	// recreate CEL environment with condition declarations
	env, err = newEnv(opts...)
	if err != nil {
		return CelCtx{}, fmt.Errorf("%w: %w", errEnvCreation, err)
	}
	cc.env = env

	return cc, nil
}

// Evaluate CEL expressions against the prepared template context and condition results.
func (cc *CelCtx) Evaluate(expr string) (bool, error) {
	return cc.evaluate(expr, defaultEnvProgram(), defaultProgramEval())
}

func defaultEnvProgram() envProgramFn {
	return func(env *cel.Env, ast *cel.Ast) (cel.Program, error) {
		return env.Program(ast)
	}
}

func defaultProgramEval() programEvalFn {
	return func(program cel.Program, ctx any) (ref.Val, *cel.EvalDetails, error) {
		return program.Eval(ctx)
	}
}

func (cc *CelCtx) evaluate(expr string, envProgram envProgramFn, programEval programEvalFn) (bool, error) {
	// compile CEL expression
	ast, issues := cc.env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return false, fmt.Errorf("%w: %w", errExpressionCompilation, issues.Err())
	}

	// create program
	program, err := envProgram(cc.env, ast)
	if err != nil {
		return false, fmt.Errorf("%w: %w", errProgramConstruction, err)
	}

	// evaluate the expression with context input
	out, _, err := programEval(program, cc.ctxMap)
	if err != nil {
		return false, fmt.Errorf("%w: %w", errProgramEvaluation, err)
	}

	// make sure that result type is 'bool'
	if !reflect.DeepEqual(out.Type(), cel.BoolType) {
		return false, fmt.Errorf("%w: %v, expected %v", errInvalidReturnType, ast.OutputType(), cel.BoolType)
	}

	return out.Value().(bool), nil
}

func unpackContext(tmplCtx *packagetypes.PackageRenderContext) (map[string]any, []cel.EnvOption, error) {
	if tmplCtx == nil {
		return map[string]any{}, []cel.EnvOption{}, nil
	}

	ctxMap, err := structToMap(tmplCtx)
	if err != nil {
		return nil, nil, fmt.Errorf("context serialization error: %w", err)
	}

	opts := make([]cel.EnvOption, 0, len(ctxMap))
	for k := range ctxMap {
		opts = append(opts, cel.Variable(k, cel.MapType(cel.StringType, cel.AnyType)))
	}

	return ctxMap, opts, nil
}

func structToMap[T any](p *T) (map[string]any, error) {
	data, err := json.Marshal(*p)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	err = json.Unmarshal(data, &result)
	return result, err
}
