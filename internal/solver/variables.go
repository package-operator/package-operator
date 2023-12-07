package solver

import (
	"fmt"
	"slices"

	"github.com/operator-framework/deppy/pkg/deppy"
)

// variable is a thing that the solver handles.
type variable struct {
	// solverIdentifier uniquely identifies this thing for the solver and
	// is set by the [Prepare] method of installation.
	solverIdentifier deppy.Identifier
	// solverConstraints contains constraints for this thing and
	// is set by the [Prepare] method of installation.
	solverConstraints []deppy.Constraint
}

func (v variable) Identifier() deppy.Identifier    { return v.solverIdentifier }
func (v variable) Constraints() []deppy.Constraint { return v.solverConstraints }

func fillIdentifiersAndLinks[IM InstallationData, SM ScopeData, CM CandidateData](i *Installation[IM, SM, CM]) {
	i.solverIdentifier = "installation"
	for si := range i.Scopes {
		s := &i.Scopes[si]
		s.installation = i
		s.solverIdentifier = s.Data.ScopeIdentifier()
		for ci := range s.Candidates {
			c := &s.Candidates[ci]
			c.scope = s
			c.solverIdentifier = c.Data.CandidateIdentifier()
		}
	}
}

func ensureNonDuplicates[IM InstallationData, SM ScopeData, CM CandidateData](i *Installation[IM, SM, CM]) {
	ids := []deppy.Identifier{i.solverIdentifier}

	add := func(v deppy.Identifier) {
		if slices.Contains(ids, v) {
			panic(fmt.Errorf("%w: identifier %q defined multiple times", ErrDatastructure, v))
		}
		ids = append(ids, v)
	}

	for _, s := range i.Scopes {
		add(s.solverIdentifier)
		for _, c := range s.Candidates {
			add(c.solverIdentifier)
		}
	}
}

func generateConstraints[IM InstallationData, SM ScopeData, CM CandidateData](i *Installation[IM, SM, CM]) {
	i.generateConstraints()
	for scopeIdx := range i.Scopes {
		i.Scopes[scopeIdx].generateConstraints()
		for candidateIdx := range i.Scopes[scopeIdx].Candidates {
			i.Scopes[scopeIdx].Candidates[candidateIdx].generateConstraints()
		}
	}
}

func InstallationAsVariables[IM InstallationData, SM ScopeData, CM CandidateData](installation Installation[IM, SM, CM]) []deppy.Variable {
	fillIdentifiersAndLinks(&installation)
	ensureNonDuplicates(&installation)
	generateConstraints(&installation)

	vars := []deppy.Variable{installation}
	for _, scope := range installation.Scopes {
		vars = append(vars, scope)
		for _, candidate := range scope.Candidates {
			vars = append(vars, candidate)
		}
	}
	return vars
}
