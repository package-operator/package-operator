package celctx

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"

	"github.com/google/cel-go/cel"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
)

var errInvalidReturnType = errors.New("invalid return type")

// CelCtx contains a cel environment that is prepared with a tmplCtx and pre-evaluated macros.
type CelCtx struct {
	env    *cel.Env
	ctxMap map[string]any
}

// New pre-evaluates the given macros against tmplCtx and exposes both tmplCtx + macro results to cel programs.
func New(macros []manifests.PackageManifestCelMacro, tmplCtx *packagetypes.PackageRenderContext) (CelCtx, error) {
	ctxMap, opts, err := unpackContext(tmplCtx)
	if err != nil {
		return CelCtx{}, fmt.Errorf("context unpacking error: %w", err)
	}

	// create CEL environment with context
	env, err := cel.NewEnv(opts...)
	if err != nil {
		return CelCtx{}, fmt.Errorf("env error: %w", err)
	}

	cc := CelCtx{
		env:    env,
		ctxMap: ctxMap,
	}

	for _, m := range macros {
		// make sure macro name is allowed
		if !macroNameRegexp.MatchString(m.Name) {
			return CelCtx{}, fmt.Errorf("%w: '%s'", ErrInvalidCELMacroName, m.Name)
		}

		// make sure name is unique and does not shadow a key in ctxMap
		if _, ok := ctxMap[m.Name]; ok {
			return CelCtx{}, fmt.Errorf("%w: '%s'", ErrDuplicateCELMacroName, m.Name)
		}

		result, err := cc.Evaluate(m.Expression)
		if err != nil {
			return CelCtx{}, fmt.Errorf("%w: '%s': %w", ErrCELMacroEvaluation, m.Name, err)
		}

		// store evaluation result in context
		ctxMap[m.Name] = result

		// store macro variable name declaration
		opts = append(opts, cel.Variable(m.Name, cel.BoolType))
	}

	// recreate CEL environment with macro declarations
	env, err = cel.NewEnv(opts...)
	if err != nil {
		return CelCtx{}, fmt.Errorf("env error: %w", err)
	}
	cc.env = env

	return cc, nil
}

// Evaluate CEL expressions against the prepared template context and macro results.
func (cc *CelCtx) Evaluate(expr string) (bool, error) {
	// compile CEL expression
	ast, issues := cc.env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return false, fmt.Errorf("compile error: %w", issues.Err())
	}

	// create program
	program, err := cc.env.Program(ast)
	if err != nil {
		return false, fmt.Errorf("program construction error: %w", err)
	}

	// evaluate the expression with context input
	out, _, err := program.Eval(cc.ctxMap)
	if err != nil {
		return false, fmt.Errorf("evaluation error: %w", err)
	}

	// make sure that result type is 'bool'
	if !reflect.DeepEqual(out.Type(), cel.BoolType) {
		return false, newErrInvalidReturnType(ast.OutputType(), cel.BoolType)
	}

	return out.Value().(bool), nil
}

func newErrInvalidReturnType(actual, expected *cel.Type) error {
	return fmt.Errorf("%w: %v, expected %v", errInvalidReturnType, actual, expected)
}

func unpackContext(tmplCtx *packagetypes.PackageRenderContext) (map[string]any, []cel.EnvOption, error) {
	// TODO ask why?
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

var (
	ErrDuplicateCELMacroName = errors.New("duplicate CEL macro name")
	ErrCELMacroEvaluation    = errors.New("CEL macro evaluation failed")
	ErrInvalidCELMacroName   = errors.New("invalid CEL macro name")
	macroNameRegexp          = regexp.MustCompile("^[_a-zA-Z][_a-zA-Z0-9]*$")
)
