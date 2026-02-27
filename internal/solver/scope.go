package solver

import (
	"github.com/operator-framework/deppy/pkg/deppy"
	"github.com/operator-framework/deppy/pkg/deppy/constraint"
)

// ScopeConstrainer is used to generate constraints for a [Scope].
type ScopeConstrainer[IM InstallationData, SM ScopeData, CM CandidateData] func(
	s ScopeAccessor[IM, SM, CM],
) []deppy.Constraint

type ScopeData interface {
	ScopeIdentifier() deppy.Identifier
}

type MockScopeData struct {
	ID string
}

func (s MockScopeData) ScopeIdentifier() deppy.Identifier {
	return deppy.Identifier("scope:" + s.ID)
}

// ScopeAccessor is used to generate constraints for an [Scope].
type ScopeAccessor[IM InstallationData, SM ScopeData, CM CandidateData] interface {
	// ScopeMetadata returns the meta data of this [Scope].
	ScopeData() SM
	// ScopeCandidateAccessors returns all [Candidate] of the [Scope].
	// Returned slice may be modified.
	ScopeCandidateAccessors() []CandidateAccessor[IM, SM, CM]
	// ScopeInstallationAccessor returns the [InstallationAccessor] to
	// the [Installation] this [Scope] belongs to.
	ScopeInstallationAccessor() InstallationAccessor[IM, SM, CM]
}

// Scope represents a scope in an [Installation].
type Scope[IM InstallationData, SM ScopeData, CM CandidateData] struct {
	variable

	// installation is a reference to the [installation] this [Scope] belongs to.
	// Automatically set by [installation].
	installation InstallationAccessor[IM, SM, CM]
	// Constrainers are called to generate solver constraints.
	Constrainers []ScopeConstrainer[IM, SM, CM]
	// Data contains arbitrary meta data.
	Data SM

	// Candidates defines all installation candidates in this scope.
	Candidates []Candidate[IM, SM, CM]
}

func (s Scope[_, SM, _]) ScopeData() SM { return s.Data }

func (s Scope[IM, SM, CM]) ScopeCandidateAccessors() []CandidateAccessor[IM, SM, CM] {
	res := make([]CandidateAccessor[IM, SM, CM], 0, len(s.Candidates))
	for _, c := range s.Candidates {
		res = append(res, c)
	}
	return res
}

func (s Scope[IM, SM, CM]) ScopeInstallationAccessor() InstallationAccessor[IM, SM, CM] {
	return s.installation
}

func (s *Scope[IM, SM, CM]) generateConstraints() {
	s.solverConstraints = []deppy.Constraint{constraint.Mandatory()}

	for _, constrainer := range s.Constrainers {
		s.solverConstraints = append(s.solverConstraints, constrainer(s)...)
	}
}
