package packagerender

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/golang/glog"
	"github.com/google/cel-go/cel"

	"package-operator.run/internal/packages/internal/packagetypes"
)

var errInvalidReturnType = errors.New("invalid return type")

func evaluateCELCondition(celExpr string, _ packagetypes.PackageRenderContext) (bool, error) {
	env, err := cel.NewEnv()
	if err != nil {
		glog.Exitf("env error: %v", err)
	}

	ast, issues := env.Compile(celExpr)
	if issues != nil && issues.Err() != nil {
		return false, fmt.Errorf("compile error: %w", issues.Err())
	}

	if !reflect.DeepEqual(ast.OutputType(), cel.BoolType) {
		return false, fmt.Errorf("%w: %v, expected %v", errInvalidReturnType, ast.OutputType(), cel.BoolType)
	}

	program, err := env.Program(ast)
	if err != nil {
		return false, fmt.Errorf("program construction error: %w", err)
	}

	out, _, err := program.Eval(cel.NoVars())
	if err != nil {
		return false, fmt.Errorf("evaluation error: %w", err)
	}

	return out.Value().(bool), nil
}
