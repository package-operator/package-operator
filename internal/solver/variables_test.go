package solver_test

import (
	"testing"

	"github.com/operator-framework/deppy/pkg/deppy"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/solver"
)

func TestInstallationPrepareAllEmpty(t *testing.T) {
	t.Parallel()

	inst := solver.Installation[struct{}, solver.MockScopeData, solver.MockCandidateData]{}
	solver.InstallationAsVariables(inst)
}

type mockInstallationData struct {
	teststr string
}

func TestPrepareCandidateSet(t *testing.T) {
	t.Parallel()

	cData := solver.MockCandidateData{ID: "yes@3.0.0"}
	sData := solver.MockScopeData{ID: "scope"}
	iData := mockInstallationData{"ohno"}

	testCandConst := func(c solver.CandidateAccessor[mockInstallationData, solver.MockScopeData, solver.MockCandidateData]) []deppy.Constraint {
		require.NotNil(t, c.CandidateScopeAccessor().ScopeInstallationAccessor())
		require.Equal(t, cData, c.CandidateData())
		return nil
	}
	testInstConst := func(i solver.InstallationAccessor[mockInstallationData, solver.MockScopeData, solver.MockCandidateData]) []deppy.Constraint {
		require.Len(t, i.InstallationScopes(), 1)
		require.NotNil(t, i.InstallationScopes()[0])
		require.Equal(t, iData, i.InstallationData())
		return nil
	}

	testScopeConst := func(s solver.ScopeAccessor[mockInstallationData, solver.MockScopeData, solver.MockCandidateData]) []deppy.Constraint {
		require.NotNil(t, s.ScopeInstallationAccessor())
		require.Len(t, s.ScopeCandidateAccessors(), 1)
		require.NotNil(t, s.ScopeCandidateAccessors()[0])
		require.Equal(t, sData, s.ScopeData())
		return nil
	}

	inst := solver.Installation[mockInstallationData, solver.MockScopeData, solver.MockCandidateData]{
		Data:         iData,
		Constrainers: []solver.InstallationConstrainer[mockInstallationData, solver.MockScopeData, solver.MockCandidateData]{testInstConst},
		Scopes: []solver.Scope[mockInstallationData, solver.MockScopeData, solver.MockCandidateData]{
			{
				Data:         sData,
				Constrainers: []solver.ScopeConstrainer[mockInstallationData, solver.MockScopeData, solver.MockCandidateData]{testScopeConst},
				Candidates: []solver.Candidate[mockInstallationData, solver.MockScopeData, solver.MockCandidateData]{{
					Data:         cData,
					Constrainers: []solver.CandidateConstrainer[mockInstallationData, solver.MockScopeData, solver.MockCandidateData]{testCandConst},
				}},
			},
		},
	}
	require.Len(t, solver.InstallationAsVariables(inst), 3)
}

func TestPrepareCandidateDuplicateVersion(t *testing.T) {
	t.Parallel()

	inst := solver.Installation[struct{}, solver.MockScopeData, solver.MockCandidateData]{
		Scopes: []solver.Scope[struct{}, solver.MockScopeData, solver.MockCandidateData]{
			{
				Candidates: []solver.Candidate[struct{}, solver.MockScopeData, solver.MockCandidateData]{
					{Data: solver.MockCandidateData{ID: "yes@1.0.0"}},
					{Data: solver.MockCandidateData{ID: "yes@1.0.0"}},
				},
			},
		},
	}

	require.Panics(t, func() { solver.InstallationAsVariables(inst) })
}
