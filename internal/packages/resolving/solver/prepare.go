package solver

import (
	"fmt"

	"github.com/operator-framework/deppy/pkg/deppy"
	"github.com/operator-framework/deppy/pkg/deppy/constraint"
)

// countFunc counts the amount of elements in s for which the func f returns true.
func countFunc[S ~[]E, E any](s S, f func(E) bool) (count int) {
	for i := range s {
		if f(s[i]) {
			count++
		}
	}

	return
}

func fillIdentifiersAndLinks[IM InstallationData, SM ScopeData, CM CandidateData](installation *Installation[IM, SM, CM]) {
	installation.solverIdentifier = "installation"
	for scopeIdx := range installation.Scopes {
		installation.Scopes[scopeIdx].installation = installation
		installation.Scopes[scopeIdx].solverIdentifier = installation.Scopes[scopeIdx].Data.ScopeIdentifier()
		for candidateIdx := range installation.Scopes[scopeIdx].Candidates {
			installation.Scopes[scopeIdx].Candidates[candidateIdx].scope = installation.Scopes[scopeIdx]
			installation.Scopes[scopeIdx].Candidates[candidateIdx].solverIdentifier = installation.Scopes[scopeIdx].Candidates[candidateIdx].Data.CandidateIdentifier()
		}
	}
}

func ensureNonDuplicates[IM InstallationData, SM ScopeData, CM CandidateData](installation *Installation[IM, SM, CM]) {
	for scopeIdx := range installation.Scopes {
		// Ensure scope uniqueness since the solver requires that that.
		count := countFunc(installation.Scopes, func(s Scope[IM, SM, CM]) bool {
			return installation.Scopes[scopeIdx].solverIdentifier == s.solverIdentifier
		})
		if count != 1 {
			panic(fmt.Errorf("%w: scope %s defined multiple times", ErrDatastructure, installation.Scopes[scopeIdx].solverIdentifier))
		}

		for candidateIdx := range installation.Scopes[scopeIdx].Candidates {
			// Ensure candidate uniqueness within a scope since the solver requires that that.
			count := countFunc(installation.Scopes[scopeIdx].Candidates, func(c Candidate[IM, SM, CM]) bool {
				return installation.Scopes[scopeIdx].Candidates[candidateIdx].solverIdentifier == c.solverIdentifier
			})
			if count != 1 {
				panic(fmt.Errorf("%w: scope %s: candidate %s defined %d times", ErrDatastructure, installation.Scopes[scopeIdx].solverIdentifier, installation.Scopes[scopeIdx].Candidates[candidateIdx].solverIdentifier, count))
			}
		}
	}
}

func generateConstraints[IM InstallationData, SM ScopeData, CM CandidateData](installation *Installation[IM, SM, CM]) {
	installation.solverConstraints = []deppy.Constraint{constraint.Mandatory()}

	// Constraints for the installation.
	for _, constrainer := range installation.Constrainers {
		installation.solverConstraints = append(installation.solverConstraints, constrainer(*installation)...)
	}

	for scopeIdx := range installation.Scopes {
		installation.Scopes[scopeIdx].solverConstraints = []deppy.Constraint{constraint.Mandatory()}

		for _, constrainer := range installation.Scopes[scopeIdx].Constrainers {
			installation.Scopes[scopeIdx].solverConstraints = append(installation.Scopes[scopeIdx].solverConstraints,
				constrainer(installation.Scopes[scopeIdx])...,
			)
		}

		for candidateIdx := range installation.Scopes[scopeIdx].Candidates {
			for _, constrainer := range installation.Scopes[scopeIdx].Candidates[candidateIdx].Constrainers {
				installation.Scopes[scopeIdx].Candidates[candidateIdx].solverConstraints = append(installation.Scopes[scopeIdx].Candidates[candidateIdx].solverConstraints,
					constrainer(installation.Scopes[scopeIdx].Candidates[candidateIdx])...,
				)
			}
		}
	}
}

// Prepare the give [Installation] for use by the solver and the set of [deppy.Variable] for it.
func Prepare[IM InstallationData, SM ScopeData, CM CandidateData](installation *Installation[IM, SM, CM]) []deppy.Variable {
	fillIdentifiersAndLinks(installation)
	ensureNonDuplicates(installation)
	generateConstraints(installation)

	vars := []deppy.Variable{}
	for scopeIdx := range installation.Scopes {
		vars = append(vars, installation.Scopes[scopeIdx])
		for candidateIdx := range installation.Scopes[scopeIdx].Candidates {
			vars = append(vars, installation.Scopes[scopeIdx].Candidates[candidateIdx])
		}
	}
	return vars
}
