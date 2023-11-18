package repo

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/Masterminds/semver/v3"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/resolving/solver"
)

// PackageReferenceFromRepoEntry generates a PackageReference from a RepositoryEntry.
func PackageReferenceFromRepoEntry(repoEntry *manifests.RepositoryEntry) string {
	return repoEntry.Name // TODO: just use name for now lol.
}

// ErrRepositoryInconsistent indicates that the [manifests.RepositoryEntry] set of a repository is inconsistent.
var ErrRepositoryInconsistent = errors.New("package repository inconsistent")

// AllowedPlatformVersionsFromRepoEntry uses the given [manifests.RepositoryEntry] to generate a set of [CandidateConstrainer] that it represents.
func AllowedPlatformVersionsFromRepoEntry(repoEntry *manifests.RepositoryEntry) (map[string]solver.VersionRange, error) {
	platformTypeAllowSets := [][]string{}
	for idx, c := range repoEntry.Data.Constraints {
		var constraintPlatformTypes []string
		for _, constraintPlatformType := range c.Platform {
			if constraintPlatformType == "" {
				return nil, fmt.Errorf("%w: entry %s constraint #%d has empty platform type in allow list", ErrRepositoryInconsistent, repoEntry.Name, idx)
			}
			constraintPlatformTypes = append(constraintPlatformTypes, string(constraintPlatformType))
		}
		if len(constraintPlatformTypes) != 0 {
			platformTypeAllowSets = append(platformTypeAllowSets, constraintPlatformTypes)
		}
	}

	knownPlatformTypes := []string{}
	for _, a := range platformTypeAllowSets {
		knownPlatformTypes = append(knownPlatformTypes, a...)
	}
	slices.Sort(knownPlatformTypes)
	knownPlatformTypes = slices.Compact(knownPlatformTypes)

	rangeWild, err := semver.NewConstraint("*")
	if err != nil {
		panic(err)
	}
	allowedPlatformVersions := map[string]solver.VersionRange{}
	for _, knownPlatformType := range knownPlatformTypes {
		knownPlatformTypeInAllAllowSets := true
		for _, platformTypeAllowSet := range platformTypeAllowSets {
			if knownPlatformTypeInAllAllowSets = slices.Contains(platformTypeAllowSet, knownPlatformType); !knownPlatformTypeInAllAllowSets {
				break
			}
		}
		if knownPlatformTypeInAllAllowSets {
			allowedPlatformVersions[knownPlatformType] = *rangeWild
		}
	}

	for idx, c := range repoEntry.Data.Constraints {
		if c.PlatformVersion != nil {
			name := string(c.PlatformVersion.Name)
			if name == "" {
				return nil, fmt.Errorf("%w: entry %s constraint #%d has empty platform type for version range", ErrRepositoryInconsistent, repoEntry.Name, idx)
			}

			con, err := semver.NewConstraint(c.PlatformVersion.Range)
			if err != nil {
				return nil, fmt.Errorf("%w: entry %s constraint #%d has invalid range: %w", ErrRepositoryInconsistent, repoEntry.Name, idx, err)
			}

			allowedPlatformVersions[name] = *con
		}
	}

	return allowedPlatformVersions, nil
}

type CandidateData struct {
	PackageName             string
	PackageVersion          solver.Version
	TargetNamespace         string
	RepoEntry               *manifests.RepositoryEntry
	AllowedPlatformVersions map[string]solver.VersionRange
}

func (c CandidateData) CandidateIdentifier() solver.Identifier {
	return solver.Identifier(fmt.Sprintf("%s:%s@%s", c.TargetNamespace, c.PackageName, c.PackageVersion.String()))
}

func (c CandidateData) CandidatePackageName() string                         { return c.PackageName }
func (c CandidateData) CandidatePackageVersion() solver.Version              { return c.PackageVersion }
func (c CandidateData) CandidateSourceRepoEntry() *manifests.RepositoryEntry { return c.RepoEntry }
func (c CandidateData) CandidateAllowedPlatformVersions() map[string]solver.VersionRange {
	return maps.Clone(c.AllowedPlatformVersions)
}

// CandidatesFromRepoEntry uses the [Candidate] in the parameter template to create Candidates to create Candidates for all versions in the given [manifests.RepositoryEntry]
// in parameter entry.
func CandidatesFromRepoEntry[IM solver.InstallationData, SM solver.ScopeData](template solver.Candidate[IM, SM, CandidateData], entry *manifests.RepositoryEntry) ([]solver.Candidate[IM, SM, CandidateData], error) {
	results := []solver.Candidate[IM, SM, CandidateData]{}
	reference := PackageReferenceFromRepoEntry(entry)

	allowedPlatformVersions, err := AllowedPlatformVersionsFromRepoEntry(entry)
	if err != nil {
		return nil, err
	}

	// Do for all versions of this entry.
	for _, verStr := range entry.Data.Versions {
		ver, err := semver.StrictNewVersion(verStr)
		if err != nil {
			return nil, err
		}

		// If the version of a package is defined multiple times  report an error.
		duplicate := slices.ContainsFunc(results, func(a solver.Candidate[IM, SM, CandidateData]) bool {
			return a.Data.PackageVersion.Equal(ver)
		})
		if duplicate {
			return nil, fmt.Errorf("%w version %s of package %s exists multiple times in repository entry", ErrRepositoryInconsistent, verStr, reference)
		}

		newCandidate := template
		newCandidate.Data = CandidateData{entry.ObjectMeta.Name, *ver, "", entry, allowedPlatformVersions}
		// Create a new constrainers slice and append template constrainers to it.
		newCandidate.Constrainers = append([]solver.CandidateConstrainer[IM, SM, CandidateData]{}, template.Constrainers...)

		results = append(results, newCandidate)
	}

	return results, nil
}
