package resolving

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/resolving/repo"
	"package-operator.run/internal/packages/resolving/solver"
)

type InstallRequest struct {
	Scope            string
	PackageReference string
	Version          solver.VersionRange
}

type InstallationResolution struct {
	Scope            string
	PackageReference string
	Version          solver.Version
	RepoEntry        *manifests.RepositoryEntry
}

type InstallationMetadata struct{}

type NamespaceData struct {
	Name string
}

func (n NamespaceData) ScopeIdentifier() solver.Identifier {
	return solver.Identifier(fmt.Sprintf("ns:%s", n.Name))
}

func SolveThings(ctx context.Context, reqs []InstallRequest, entries []*manifests.RepositoryEntry) ([]InstallationResolution, error) {
	candidates := []solver.Candidate[InstallationMetadata, NamespaceData, repo.CandidateData]{}

	template := solver.Candidate[InstallationMetadata, NamespaceData, repo.CandidateData]{}
	for _, entry := range entries {
		entryCandidates, err := repo.CandidatesFromRepoEntry(template, entry)
		if err != nil {
			return nil, err
		}
		for _, candidate := range entryCandidates {
			alreadyPresent := slices.ContainsFunc(candidates, func(a solver.Candidate[InstallationMetadata, NamespaceData, repo.CandidateData]) bool {
				return a.Data.CandidateIdentifier() == candidate.Data.CandidateIdentifier()
			})
			if alreadyPresent {
				return nil, fmt.Errorf("%w: different repo entries define a package with the same name %s and version %s", repo.ErrRepositoryInconsistent, candidate.Data.PackageName, candidate.Data.PackageVersion.String())
			}
		}
	}

	slices.SortFunc(candidates, func(a, b solver.Candidate[InstallationMetadata, NamespaceData, repo.CandidateData]) int {
		versionOrder := b.Data.PackageVersion.Compare(&a.Data.PackageVersion)
		if versionOrder != 0 {
			return versionOrder
		}

		return strings.Compare(a.Data.PackageName, b.Data.PackageName)
	})

	installation := solver.Installation[InstallationMetadata, NamespaceData, repo.CandidateData]{
		Scopes: []solver.Scope[InstallationMetadata, NamespaceData, repo.CandidateData]{},
	}

	for _, req := range reqs {
		var scope *solver.Scope[InstallationMetadata, NamespaceData, repo.CandidateData]
		for scopeIdx := range installation.Scopes {
			if installation.Scopes[scopeIdx].Data.Name == req.Scope {
				scope = &installation.Scopes[scopeIdx]
			}
		}
		if scope == nil {
			s := solver.Scope[InstallationMetadata, NamespaceData, repo.CandidateData]{
				Data:       NamespaceData{Name: req.Scope},
				Candidates: candidates,
			}
			installation.Scopes = append(installation.Scopes, s)
			scope = &installation.Scopes[len(installation.Scopes)-1]
		}
		scope.Constrainers = append(scope.Constrainers, repo.ScopeInstallsCandidate[InstallationMetadata, NamespaceData, repo.CandidateData](req.PackageReference, req.Version))
	}

	slices.SortFunc(installation.Scopes, func(a, b solver.Scope[InstallationMetadata, NamespaceData, repo.CandidateData]) int {
		return strings.Compare(a.Data.Name, b.Data.Name)
	})

	solution, err := solver.Solve(ctx, installation)
	if err != nil {
		return nil, err
	}

	resolutions := []InstallationResolution{}
	for _, s := range solution {
		meta := s.Data
		resolutions = append(resolutions, InstallationResolution{meta.TargetNamespace, meta.PackageName, meta.PackageVersion, meta.RepoEntry})
	}

	return resolutions, nil
}
