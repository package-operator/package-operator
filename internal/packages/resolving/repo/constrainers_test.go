package repo_test

import (
	"context"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages/resolving/repo"
	"package-operator.run/internal/packages/resolving/solver"
)

type Platform struct {
	Type    string
	Version solver.Version
}

func (p Platform) PlatformType() string            { return p.Type }
func (p Platform) PlatformVersion() solver.Version { return p.Version }

func mustVersion(s string) solver.Version {
	v, err := semver.StrictNewVersion(s)
	if err != nil {
		panic(err)
	}
	return *v
}

func mustVersionRange(s string) solver.VersionRange {
	c, err := semver.NewConstraint(s)
	if err != nil {
		panic(err)
	}
	return *c
}

type (
	MIM = solver.MockInstallationData
	MSM = solver.MockScopeData
	MCM = solver.MockCandidateData
)

func TestInstallationForbids(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inst := solver.Installation[MIM, MSM, repo.CandidateData]{
		Constrainers: []solver.InstallationConstrainer[MIM, MSM, repo.CandidateData]{
			repo.InstallationForbidsCandidate[MIM, MSM, repo.CandidateData]("budgie", mustVersionRange(">1.0.0")),
		},
		Scopes: []solver.Scope[MIM, MSM, repo.CandidateData]{
			{
				Data: solver.MockScopeData{},
				Constrainers: []solver.ScopeConstrainer[MIM, MSM, repo.CandidateData]{
					repo.ScopeInstallsCandidate[MIM, MSM, repo.CandidateData]("budgie", mustVersionRange("*")),
				},
				Candidates: []solver.Candidate[MIM, MSM, repo.CandidateData]{
					{Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("3.0.0")}},
					{Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("2.0.0")}},
					{Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("1.0.0")}},
				},
			},
		},
	}

	candidates, err := solver.Solve(ctx, inst)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, "1.0.0", candidates[0].Data.PackageVersion.String())
}

func TestScopeForbids(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inst := solver.Installation[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
		Scopes: []solver.Scope[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
			{
				Constrainers: []solver.ScopeConstrainer[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
					repo.ScopeForbidsCandidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]("budgie", mustVersionRange(">1.0.0")),
					repo.ScopeInstallsCandidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]("budgie", mustVersionRange("*")),
				},
				Candidates: []solver.Candidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
					{Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("3.0.0")}},
					{Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("2.0.0")}},
					{Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("1.0.0")}},
				},
			},
		},
	}

	candidates, err := solver.Solve(ctx, inst)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, "1.0.0", candidates[0].Data.PackageVersion.String())
}

func TestSolveEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, err := solver.Solve(ctx, solver.Installation[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{})
	require.NoError(t, err)
}

func TestSolvePrerelease(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inst := solver.Installation[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
		Scopes: []solver.Scope[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
			{
				Constrainers: []solver.ScopeConstrainer[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
					repo.ScopeInstallsCandidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]("budgie", mustVersionRange("<3-0")),
				},
				Candidates: []solver.Candidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
					{Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("3.0.0-burger.4")}},
					{Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("2.0.0-cheese.3")}},
					{Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("1.0.0")}},
				},
			},
		},
	}

	candidates, err := solver.Solve(ctx, inst)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, "2.0.0-cheese.3", candidates[0].Data.PackageVersion.String())
}

func TestSolveZeroCandidatesForDependency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inst := solver.Installation[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
		Scopes: []solver.Scope[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
			{
				Constrainers: []solver.ScopeConstrainer[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
					repo.ScopeInstallsCandidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]("budgie", mustVersionRange("<3")),
				},
				Candidates: []solver.Candidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
					{
						Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("1.0.0")},
						Constrainers: []solver.CandidateConstrainer[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
							repo.CandidateDependsOnCandidateInSameScope[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]("notthere", mustVersionRange("*")),
						},
					},
				},
			},
		},
	}

	_, err := solver.Solve(ctx, inst)
	require.Error(t, err)
}

func TestSolveNoSelectCandidatesForDependency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inst := solver.Installation[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
		Scopes: []solver.Scope[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
			{
				Constrainers: []solver.ScopeConstrainer[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
					repo.ScopeInstallsCandidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]("budgie", mustVersionRange("<3")),
				},
				Candidates: []solver.Candidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
					{
						Constrainers: []solver.CandidateConstrainer[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
							repo.CandidateDependsOnCandidateInSameScope[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]("perch", mustVersionRange(">2")),
						},
					},
					{Data: repo.CandidateData{PackageName: "perch", PackageVersion: mustVersion("2.0.0")}},
					{Data: repo.CandidateData{PackageName: "perch", PackageVersion: mustVersion("1.0.0")}},
				},
			},
		},
	}

	_, err := solver.Solve(ctx, inst)
	require.Error(t, err)
}

func TestSingleVersionInScope(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inst := solver.Installation[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
		Scopes: []solver.Scope[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
			{
				Constrainers: []solver.ScopeConstrainer[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
					repo.ScopeInstallsCandidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]("budgie", mustVersionRange("=1")),
					repo.ScopeInstallsCandidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]("budgie", mustVersionRange("=2")),
				},
				Candidates: []solver.Candidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
					{
						Data:         repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("2.0.0")},
						Constrainers: []solver.CandidateConstrainer[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{repo.SingleCandidateVersionInScope[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]},
					},
					{
						Data:         repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("1.0.0")},
						Constrainers: []solver.CandidateConstrainer[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{repo.SingleCandidateVersionInScope[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]},
					},
				},
			},
		},
	}

	_, err := solver.Solve(ctx, inst)
	require.Error(t, err)
}

func TestCandidateConflictsWith(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inst := solver.Installation[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
		Scopes: []solver.Scope[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
			{
				Constrainers: []solver.ScopeConstrainer[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
					repo.ScopeInstallsCandidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]("budgie", mustVersionRange("=1")),
					repo.ScopeInstallsCandidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]("budgie", mustVersionRange("=2")),
				},
				Candidates: []solver.Candidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
					{
						Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("2.0.0")},
						Constrainers: []solver.CandidateConstrainer[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
							repo.CandidateConflictsWithCandidateInSameScope[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]("budgie", mustVersionRange("=1")),
						},
					},
					{Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("1.0.0")}},
				},
			},
		},
	}

	_, err := solver.Solve(ctx, inst)
	require.Error(t, err)
}

func TestSolvePlatform(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inst := solver.Installation[Platform, MSM, repo.CandidateData]{
		Data: Platform{Type: "helpusblobivonkablobi", Version: mustVersion("3.3.3")},
		Scopes: []solver.Scope[Platform, MSM, repo.CandidateData]{
			{
				Constrainers: []solver.ScopeConstrainer[Platform, MSM, repo.CandidateData]{
					repo.ScopeInstallsCandidate[Platform, MSM, repo.CandidateData]("budgie", mustVersionRange("*")),
				},
				Candidates: []solver.Candidate[Platform, MSM, repo.CandidateData]{
					{
						Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("3.0.0")},
						Constrainers: []solver.CandidateConstrainer[Platform, MSM, repo.CandidateData]{
							repo.RequireInstallationPlatformVersionToBeOneOf[Platform, MSM, repo.CandidateData](map[string]solver.VersionRange{
								"helpusblobivonkablobi": mustVersionRange(">3"),
							}),
						},
					},
					{
						Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("2.0.0")},
						Constrainers: []solver.CandidateConstrainer[Platform, MSM, repo.CandidateData]{
							repo.RequireInstallationPlatformVersionToBeOneOf[Platform, MSM, repo.CandidateData](map[string]solver.VersionRange{
								"helpusblobivonkablobi": mustVersionRange("*"),
							}),
						},
					},
					{Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("1.0.0")}},
				},
			},
		},
	}

	candidates, err := solver.Solve(ctx, inst)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, "2.0.0", candidates[0].Data.PackageVersion.String())
}

func TestSolveSimple(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	prob := solver.Installation[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
		Scopes: []solver.Scope[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
			{
				Constrainers: []solver.ScopeConstrainer[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
					repo.ScopeInstallsCandidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]("budgie", mustVersionRange(">1")),
				},
				Candidates: []solver.Candidate[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
					{
						Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("3.1.1")},
						Constrainers: []solver.CandidateConstrainer[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
							repo.CandidateDependsOnCandidateInSameScope[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]("perch", mustVersionRange("*")),
						},
					},
					{
						Data: repo.CandidateData{PackageName: "budgie", PackageVersion: mustVersion("2.1.1")},
						Constrainers: []solver.CandidateConstrainer[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]{
							repo.CandidateDependsOnCandidateInSameScope[solver.MockInstallationData, solver.MockScopeData, repo.CandidateData]("hawk", mustVersionRange("*")),
						},
					},
					{Data: repo.CandidateData{PackageName: "perch", PackageVersion: mustVersion("1.0.0")}},
				},
			},
		},
	}

	solution, err := solver.Solve(ctx, prob)
	require.NoError(t, err)
	require.Len(t, solution, 2)
	require.Equal(t, ":budgie@3.1.1", string(solution[0].Data.CandidateIdentifier()))
	require.Equal(t, ":perch@1.0.0", string(solution[1].Data.CandidateIdentifier()))
}
