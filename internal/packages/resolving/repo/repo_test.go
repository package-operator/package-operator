package repo_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/resolving/repo"
	"package-operator.run/internal/packages/resolving/solver"
)

func TestRepoEntryInstallationConstraintsFromRepoEntry(t *testing.T) {
	t.Parallel()

	entry := &manifests.RepositoryEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "bunki"},
		Data: manifests.RepositoryEntryData{
			Constraints: []manifests.PackageManifestConstraint{
				{
					PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{Name: "g", Range: "5"},
					Platform:        []manifests.PlatformName{"a", "b", "c"},
				},
				{
					Platform: []manifests.PlatformName{"d"},
				},
				{},
			},
		},
	}
	res, err := repo.AllowedPlatformVersionsFromRepoEntry(entry)
	require.NoError(t, err)
	require.NotEmpty(t, res)
}

func TestRepoEntryInstallationConstraintsFromRepoEntryEmptyPlatform(t *testing.T) {
	t.Parallel()

	entry := &manifests.RepositoryEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "bunki"},
		Data: manifests.RepositoryEntryData{
			Constraints: []manifests.PackageManifestConstraint{
				{
					Platform: []manifests.PlatformName{"a", "", "c"},
				},
			},
		},
	}

	_, err := repo.AllowedPlatformVersionsFromRepoEntry(entry)
	require.Error(t, err)
}

func TestRepoEntryInstallationConstraintsFromRepoEntryEmptyVersionPlatform(t *testing.T) {
	t.Parallel()

	entry := &manifests.RepositoryEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "bunki"},
		Data: manifests.RepositoryEntryData{
			Constraints: []manifests.PackageManifestConstraint{
				{
					PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{Name: "", Range: "3"},
				},
			},
		},
	}

	_, err := repo.AllowedPlatformVersionsFromRepoEntry(entry)
	require.Error(t, err)
}

func TestRepoEntryInstallationConstraintsFromRepoEntryEmptyVersion(t *testing.T) {
	t.Parallel()

	entry := &manifests.RepositoryEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "bunki"},
		Data: manifests.RepositoryEntryData{
			Constraints: []manifests.PackageManifestConstraint{
				{
					PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{Name: "a", Range: ""},
				},
			},
		},
	}

	_, err := repo.AllowedPlatformVersionsFromRepoEntry(entry)
	require.Error(t, err)
}

func TestCandidatesFromRepoEntries(t *testing.T) {
	t.Parallel()

	entry := &manifests.RepositoryEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "bunki"},
		Data: manifests.RepositoryEntryData{
			Versions: []string{"1.1.1", "3.3.3"},
		},
	}

	template := solver.Candidate[solver.InstallationData, solver.ScopeData, repo.CandidateData]{
		Constrainers: []solver.CandidateConstrainer[solver.InstallationData, solver.ScopeData, repo.CandidateData]{nil},
	}
	candidates, err := repo.CandidatesFromRepoEntry(template, entry)
	require.NoError(t, err)

	require.Len(t, candidates, 2)

	require.Empty(t, candidates[0].Data.AllowedPlatformVersions)
	require.Empty(t, candidates[1].Data.AllowedPlatformVersions)

	require.Same(t, entry, candidates[0].Data.RepoEntry)
	require.Same(t, entry, candidates[1].Data.RepoEntry)

	require.Empty(t, candidates[0].Data.TargetNamespace)
	require.Empty(t, candidates[1].Data.TargetNamespace)

	candidates[0].Data.TargetNamespace = "spaceA"
	require.Contains(t, candidates[0].Data.CandidateIdentifier(), "1.1.1")
	require.Contains(t, candidates[0].Data.CandidateIdentifier(), "bunki")
	require.Contains(t, candidates[0].Data.CandidateIdentifier(), "spaceA")

	candidates[1].Data.TargetNamespace = "spaceB"
	require.Contains(t, candidates[1].Data.CandidateIdentifier(), "3.3.3")
	require.Contains(t, candidates[1].Data.CandidateIdentifier(), "bunki")
	require.Contains(t, candidates[1].Data.CandidateIdentifier(), "spaceB")
}

func TestCandidatesFromRepoDuplicates(t *testing.T) {
	t.Parallel()

	entry := &manifests.RepositoryEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "bunki"},
		Data: manifests.RepositoryEntryData{
			Versions: []string{"1.1.1", "1.1.1"},
		},
	}

	template := solver.Candidate[solver.InstallationData, solver.ScopeData, repo.CandidateData]{}
	_, err := repo.CandidatesFromRepoEntry(template, entry)
	require.Error(t, err)
}

func TestCandidatesFromRepoDuplicatesInvalidConstraint(t *testing.T) {
	t.Parallel()

	entry := &manifests.RepositoryEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "bunki"},
		Data: manifests.RepositoryEntryData{
			Versions:    []string{"1.1.1"},
			Constraints: []manifests.PackageManifestConstraint{{Platform: []manifests.PlatformName{""}}},
		},
	}

	template := solver.Candidate[solver.InstallationData, solver.ScopeData, repo.CandidateData]{}
	_, err := repo.CandidatesFromRepoEntry(template, entry)
	require.Error(t, err)
}

func TestCandidatesFromRepoDuplicatesInvalidVersion(t *testing.T) {
	t.Parallel()

	entry := &manifests.RepositoryEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "bunki"},
		Data: manifests.RepositoryEntryData{
			Versions: []string{""},
		},
	}

	template := solver.Candidate[solver.InstallationData, solver.ScopeData, repo.CandidateData]{}
	_, err := repo.CandidatesFromRepoEntry(template, entry)
	require.Error(t, err)
}
