package solver

import (
	"fmt"
	"slices"
	"strings"

	"github.com/operator-framework/deppy/pkg/deppy/solver"
)

// Solve the given [Installation] and return a list of [Candidate] that need to be installed.
// Calls [Prepare] on the given Installation before solving.
// If the Installation does not have required packages no candidates and no error is returned.
func Solve[IM InstallationData, SM ScopeData, CM CandidateData](installation Installation[IM, SM, CM]) ([]Candidate[IM, SM, CM], error) {
	// Run solver.
	solution, err := solver.NewDeppySolver().Solve(Prepare(&installation))
	if err != nil {
		return nil, err
	}

	// Just bail with solution error if it is set.
	if err := solution.Error(); err != nil {
		return nil, err
	}

	// Extract selected Candidates.
	candidates := []Candidate[IM, SM, CM]{}
	for _, someVariable := range solution.SelectedVariables() {
		switch variable := someVariable.(type) {
		case Installation[IM, SM, CM]:
		case Scope[IM, SM, CM]:
		case Candidate[IM, SM, CM]:
			candidates = append(candidates, variable)
		default:
			panic(fmt.Sprintf("unknown type: %T", someVariable))
		}
	}

	// Sort candidates by solver identifier to be deterministic.
	slices.SortFunc(candidates, func(a, b Candidate[IM, SM, CM]) int {
		return strings.Compare(a.Identifier().String(), b.Identifier().String())
	})

	return candidates, nil
}
