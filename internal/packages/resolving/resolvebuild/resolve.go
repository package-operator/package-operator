package resolvebuild

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"pkg.package-operator.run/semver"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/resolving/solver"
)

type Resolver struct {
	Loader RepoLoader
	inst   solver.Installation[struct{}, scopeData, candidateData]
}

func (r *Resolver) addScope(ctx context.Context, pkg *manifests.PackageManifest) error {
	mgr := defaultRepoLoaderIfNil(r.Loader)

	platform, err := platformsFromConstraints(pkg.Spec.Constraints)
	scope := solver.Scope[struct{}, scopeData, candidateData]{Data: scopeData{pkg, platform}}
	if err != nil {
		return err
	}

	for _, dep := range pkg.Spec.Dependencies {
		if dep.Image.Range != "" {
			rng, err := semver.NewConstraint(dep.Image.Range)
			if err != nil {
				return err
			}
			scope.Constrainers = append(scope.Constrainers, depdendOnConstrained(dep.Image.Package, rng))
		} else {
			scope.Constrainers = append(scope.Constrainers, depdendOUncConstrained(dep.Image.Package))
		}
	}

	idx, err := mgr(ctx, pkg.Spec.Repositories)
	if err != nil {
		return err
	}

	for _, entry := range idx.ListAllEntries() {
		allowedPlatformVersions, err := platformsFromConstraints(entry.Data.Constraints)
		if err != nil {
			return err
		}

		// Do for all versions of this entry.
		for _, verStr := range entry.Data.Versions {
			ver, err := semver.NewVersion(strings.TrimPrefix(verStr, "v"))
			if err != nil {
				return err
			}

			newCandidate := solver.Candidate[struct{}, scopeData, candidateData]{
				Data: candidateData{pkg, entry, entry.FQDN(), ver, allowedPlatformVersions},
				Constrainers: []solver.CandidateConstrainer[struct{}, scopeData, candidateData]{
					uniqueInScope,
					platformMatches,
				},
			}
			chk := func(a solver.Candidate[struct{}, scopeData, candidateData]) bool {
				return a.Data.fqdn == newCandidate.Data.fqdn && a.Data.version.Equal(newCandidate.Data.version)
			}

			// If the version of a package is defined multiple times  report an error.
			if slices.ContainsFunc(scope.Candidates, chk) {
				return fmt.Errorf("%w version %v of package %s exists multiple times in repository entry", ErrRepositoryInconsistent, newCandidate.Data.version, newCandidate.Data.fqdn)
			}
			scope.Candidates = append(scope.Candidates, newCandidate)
		}
	}

	chk := func(a, b solver.Candidate[struct{}, scopeData, candidateData]) int {
		strcmp := strings.Compare(a.Data.fqdn, b.Data.fqdn)
		if strcmp != 0 {
			return strcmp
		}

		return b.Data.version.Compare(a.Data.version)
	}

	slices.SortFunc(scope.Candidates, chk)

	r.inst.Scopes = append(r.inst.Scopes, scope)

	return nil
}

// NOTE: Since the kubectl package update cmd is unaware of subcomponents this is ignoring them too.
func (r Resolver) ResolveBuild(ctx context.Context, pkg *manifests.PackageManifest) ([]manifests.PackageManifestLockDependency, error) {
	if err := r.addScope(ctx, pkg); err != nil {
		return nil, fmt.Errorf("creating solving scope: %w", err)
	}

	deps, err := solver.Solve(r.inst)
	if err != nil {
		return nil, fmt.Errorf("solving package deps: %w", err)
	}

	res := []manifests.PackageManifestLockDependency{}
	for _, dep := range deps {
		depData := dep.CandidateData()
		for _, originalDep := range pkg.Spec.Dependencies {
			if originalDep.Image.Package == depData.fqdn {
				pkgDep := manifests.PackageManifestLockDependency{
					Name:    originalDep.Image.Name,
					Image:   depData.entry.Data.Image,
					Digest:  depData.entry.Data.Digest,
					Version: depData.version.String(),
				}
				res = append(res, pkgDep)
			}
		}
	}

	return res, nil
}
