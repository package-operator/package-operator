package solver_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages/resolving/solver"
)

func TestInstallationPrepareAllEmpty(t *testing.T) {
	t.Parallel()

	inst := &solver.Installation[solver.MockInstallationData, solver.MockScopeData, solver.MockCandidateData]{}
	solver.Prepare(inst)
}

func TestPrepareCandidateSet(t *testing.T) {
	t.Parallel()

	inst := &solver.Installation[solver.MockInstallationData, solver.MockScopeData, solver.MockCandidateData]{
		Scopes: []solver.Scope[solver.MockInstallationData, solver.MockScopeData, solver.MockCandidateData]{
			{
				Data: solver.MockScopeData{ID: "scope"},
				Candidates: []solver.Candidate[solver.MockInstallationData, solver.MockScopeData, solver.MockCandidateData]{
					{Data: solver.MockCandidateData{ID: "yes@3.0.0"}},
				},
			},
		},
	}
	solver.Prepare(inst)
	require.NotEmpty(t, inst.Identifier())
	require.NotEmpty(t, inst.Scopes[0].Identifier())
	require.NotNil(t, inst.Scopes[0].Candidates[0].CandidateScopeAccessor().ScopeData().ID)
	require.NotEmpty(t, inst.Scopes[0].Candidates[0].Identifier())
}

func TestPrepareCandidateDuplicateVersion(t *testing.T) {
	t.Parallel()

	inst := &solver.Installation[solver.MockInstallationData, solver.MockScopeData, solver.MockCandidateData]{
		Scopes: []solver.Scope[solver.MockInstallationData, solver.MockScopeData, solver.MockCandidateData]{
			{
				Candidates: []solver.Candidate[solver.MockInstallationData, solver.MockScopeData, solver.MockCandidateData]{
					{Data: solver.MockCandidateData{ID: "yes@1.0.0"}},
					{Data: solver.MockCandidateData{ID: "yes@1.0.0"}},
				},
			},
		},
	}

	require.Panics(t, func() { solver.Prepare(inst) })

	inst = &solver.Installation[solver.MockInstallationData, solver.MockScopeData, solver.MockCandidateData]{
		Scopes: []solver.Scope[solver.MockInstallationData, solver.MockScopeData, solver.MockCandidateData]{
			{
				Data: solver.MockScopeData{ID: "scopeA"},
				Candidates: []solver.Candidate[solver.MockInstallationData, solver.MockScopeData, solver.MockCandidateData]{
					{Data: solver.MockCandidateData{ID: "yes@1.0.0"}},
				},
			},
			{
				Data: solver.MockScopeData{ID: "scopeB"},
				Candidates: []solver.Candidate[solver.MockInstallationData, solver.MockScopeData, solver.MockCandidateData]{
					{Data: solver.MockCandidateData{ID: "yes@1.0.0"}},
				},
			},
		},
	}

	require.NotPanics(t, func() { solver.Prepare(inst) })
}
