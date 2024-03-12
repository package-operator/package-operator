package packagerender

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"

	"package-operator.run/internal/packages/internal/packagetypes"
)

var errInvalidReturnType = errors.New("invalid return type")

func newErrInvalidReturnType(actual, expected *cel.Type) error {
	return fmt.Errorf("%w: %v, expected %v", errInvalidReturnType, actual, expected)
}

func evaluateCELCondition(celExpr string, tmplCtx *packagetypes.PackageRenderContext) (bool, error) {
	// generate input to CEL program and environment from context
	ctxMap, opts, err := unpackContext(tmplCtx)
	if err != nil {
		return false, fmt.Errorf("context unpacking error: %w", err)
	}

	// create CEL environment with context
	env, err := cel.NewEnv(opts...)
	if err != nil {
		return false, fmt.Errorf("env error: %w", err)
	}

	// compile CEL expression
	ast, issues := env.Compile(celExpr)
	if issues != nil && issues.Err() != nil {
		return false, fmt.Errorf("compile error: %w", issues.Err())
	}

	// create program
	program, err := env.Program(ast)
	if err != nil {
		return false, fmt.Errorf("program construction error: %w", err)
	}

	// evaluate the expression with context input
	out, _, err := program.Eval(ctxMap)
	if err != nil {
		return false, fmt.Errorf("evaluation error: %w", err)
	}

	// make sure that result type is 'bool'
	if !reflect.DeepEqual(out.Type(), cel.BoolType) {
		return false, newErrInvalidReturnType(ast.OutputType(), cel.BoolType)
	}

	return out.Value().(bool), nil
}

func unpackContext(tmplCtx *packagetypes.PackageRenderContext) (map[string]any, []cel.EnvOption, error) {
	if tmplCtx == nil {
		return nil, nil, nil
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
