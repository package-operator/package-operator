package solver

import (
	"github.com/operator-framework/deppy/pkg/deppy"
)

type (
	// Constraint is a constraint that is used for solving installation sets.
	Constraint = deppy.Constraint
	// Identifier uniquely identifies a solver variable.
	Identifier = deppy.Identifier
)

// variable is a thing that the solver handles.
type variable struct {
	// solverIdentifier uniquely identifies this thing for the solver and
	// is set by the [Prepare] method of installation.
	solverIdentifier Identifier
	// solverConstraints contains constraints for this thing and
	// is set by the [Prepare] method of installation.
	solverConstraints []Constraint
}

func (v variable) Identifier() Identifier    { return v.solverIdentifier }
func (v variable) Constraints() []Constraint { return v.solverConstraints }
