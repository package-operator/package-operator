package probing

import (
	"errors"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apiserver/pkg/cel/library"
	"pkg.package-operator.run/boxcutter/machinery/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CELProbe uses the common expression language for probing.
type CELProbe struct {
	Program cel.Program
	Message string
}

var _ Prober = (*CELProbe)(nil)

// ErrCELInvalidEvaluationType is raised when a CEL expression does not evaluate to a boolean.
var ErrCELInvalidEvaluationType = errors.New("cel expression must evaluate to a bool")

// NewCELProbe creates a new CEL (Common Expression Language) Probe.
// A CEL probe runs a CEL expression against the target object that needs to evaluate to a bool.
func NewCELProbe(rule, message string) (
	*CELProbe, error,
) {
	env, err := cel.NewEnv(
		cel.Variable("self", cel.DynType),
		cel.HomogeneousAggregateLiterals(),
		cel.EagerlyValidateDeclarations(true),
		cel.DefaultUTCTimeZone(true),

		ext.Strings(ext.StringsVersion(0)),
		library.URLs(),
		library.Regex(),
		library.Lists(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating CEL env: %w", err)
	}

	ast, issues := env.Compile(rule)
	if issues != nil {
		return nil, fmt.Errorf("compiling CEL: %w", issues.Err())
	}
	if ast.OutputType() != cel.BoolType {
		return nil, ErrCELInvalidEvaluationType
	}

	prgm, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("CEL program failed: %w", err)
	}

	return &CELProbe{
		Program: prgm,
		Message: message,
	}, nil
}

// Probe executes the probe.
func (p *CELProbe) Probe(obj client.Object) types.ProbeResult {
	return probeUnstructuredSingleMsg(obj, p.probe)
}

func (p *CELProbe) probe(obj *unstructured.Unstructured) (success bool, message string) {
	val, _, err := p.Program.Eval(map[string]any{
		"self": obj.Object,
	})
	if err != nil {
		return false, fmt.Sprintf("CEL program failed: %v", err)
	}

	return val.Value().(bool), p.Message
}
