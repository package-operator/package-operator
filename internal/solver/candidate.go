package solver

import (
	"github.com/operator-framework/deppy/pkg/deppy"
)

// CandidateConstrainer is used to generate constraints for a [Candidate].
type CandidateConstrainer[IM InstallationData, SM ScopeData, CM CandidateData] func(
	c CandidateAccessor[IM, SM, CM],
) []deppy.Constraint

type CandidateData interface {
	CandidateIdentifier() deppy.Identifier
}

type MockCandidateData struct {
	ID string
}

func (c MockCandidateData) CandidateIdentifier() deppy.Identifier {
	return deppy.Identifier("candidate:" + c.ID)
}

// CandidateAccessor is used to access a [Candidate] read only.
type CandidateAccessor[IM InstallationData, SM ScopeData, CM CandidateData] interface {
	// CandidateScopeAccessor returns a [ScopeAccessor] for the [Scope] that contains this [Candidate].
	CandidateScopeAccessor() ScopeAccessor[IM, SM, CM]
	// CandidateMetadata returns the meta data of the [Candidate].
	CandidateData() CM
}

// Candidate represents a specific version of a package that can be installed.
type Candidate[IM InstallationData, SM ScopeData, CM CandidateData] struct {
	variable

	// scope is a reference to the [scope] this [Candidate] belongs to.
	// Automatically set by [Installation].
	scope ScopeAccessor[IM, SM, CM]

	// Data contains arbitrary meta data.
	Data CM

	// Constrainers are called on this Candidate to generate [SolverConstraint].
	Constrainers []CandidateConstrainer[IM, SM, CM]
}

func (c Candidate[_, _, CM]) CandidateData() CM                                   { return c.Data }
func (c Candidate[IM, SM, CM]) CandidateScopeAccessor() ScopeAccessor[IM, SM, CM] { return c.scope }

func (c *Candidate[IM, SM, CM]) generateConstraints() {
	for _, constrainer := range c.Constrainers {
		c.solverConstraints = append(c.solverConstraints, constrainer(c)...)
	}
}
