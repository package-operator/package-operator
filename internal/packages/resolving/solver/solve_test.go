package solver_test

import (
	"testing"

	"github.com/operator-framework/deppy/pkg/deppy"
	"github.com/operator-framework/deppy/pkg/deppy/constraint"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages/resolving/solver"
)

func TestSolveSuccess(t *testing.T) {
	t.Parallel()

	c := func(s solver.ScopeAccessor[struct{}, solver.MockScopeData, solver.MockCandidateData]) []deppy.Constraint {
		return []deppy.Constraint{constraint.Dependency(s.ScopeCandidateAccessors()[0].CandidateData().CandidateIdentifier())}
	}

	inst := solver.Installation[struct{}, solver.MockScopeData, solver.MockCandidateData]{
		Scopes: []solver.Scope[struct{}, solver.MockScopeData, solver.MockCandidateData]{
			{
				Data:         solver.MockScopeData{ID: "a"},
				Constrainers: []solver.ScopeConstrainer[struct{}, solver.MockScopeData, solver.MockCandidateData]{c},
				Candidates: []solver.Candidate[struct{}, solver.MockScopeData, solver.MockCandidateData]{{
					Data: solver.MockCandidateData{"b"},
				}},
			},
		},
	}

	s, err := solver.Solve(inst)
	require.NoError(t, err)
	require.Len(t, s, 1)
	require.Equal(t, inst.Scopes[0].Candidates[0].Data, s[0].Data)
}

func TestSolveSortResults(t *testing.T) {
	t.Parallel()

	c := func(s solver.ScopeAccessor[struct{}, solver.MockScopeData, solver.MockCandidateData]) []deppy.Constraint {
		return []deppy.Constraint{
			constraint.Dependency(s.ScopeCandidateAccessors()[0].CandidateData().CandidateIdentifier()),
			constraint.Dependency(s.ScopeCandidateAccessors()[1].CandidateData().CandidateIdentifier()),
		}
	}

	inst := solver.Installation[struct{}, solver.MockScopeData, solver.MockCandidateData]{
		Scopes: []solver.Scope[struct{}, solver.MockScopeData, solver.MockCandidateData]{
			{
				Data:         solver.MockScopeData{ID: "a"},
				Constrainers: []solver.ScopeConstrainer[struct{}, solver.MockScopeData, solver.MockCandidateData]{c},
				Candidates: []solver.Candidate[struct{}, solver.MockScopeData, solver.MockCandidateData]{
					{Data: solver.MockCandidateData{"b"}},
					{Data: solver.MockCandidateData{"c"}},
				},
			},
		},
	}

	s, err := solver.Solve(inst)
	require.NoError(t, err)
	require.Len(t, s, 2)
	require.Equal(t, inst.Scopes[0].Candidates[0].Data, s[0].Data)
	require.Equal(t, inst.Scopes[0].Candidates[1].Data, s[1].Data)
}

func TestSolveFail(t *testing.T) {
	t.Parallel()

	c := func(s solver.ScopeAccessor[struct{}, solver.MockScopeData, solver.MockCandidateData]) []deppy.Constraint {
		return []deppy.Constraint{constraint.Prohibited()}
	}

	inst := solver.Installation[struct{}, solver.MockScopeData, solver.MockCandidateData]{
		Scopes: []solver.Scope[struct{}, solver.MockScopeData, solver.MockCandidateData]{
			{
				Data:         solver.MockScopeData{ID: "a"},
				Constrainers: []solver.ScopeConstrainer[struct{}, solver.MockScopeData, solver.MockCandidateData]{c},
				Candidates: []solver.Candidate[struct{}, solver.MockScopeData, solver.MockCandidateData]{{
					Data: solver.MockCandidateData{"b"},
				}},
			},
		},
	}

	s, err := solver.Solve(inst)
	require.Error(t, err)
	require.Empty(t, s)
}
