package packageresolving

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"pkg.package-operator.run/semver"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/solver"
)

// ErrRepositoryInconsistent indicates that something within a package repository is inconsistent.
var ErrRepositoryInconsistent = errors.New("package repository inconsistent")

// BuildResolver resolves dependencies when building a package.
type BuildResolver struct {
	// Loader pulls package repositories.
	// Intended for mock testing, nil value tells the resolver to use the default loader.
	Loader RepoLoader
	// inst is the generated solver installation.
	inst solver.Installation[struct{}, buildSD, buildCD]
}

// AddManifest adds a manifest to be solved as scope to the solver problem.
func (r *BuildResolver) AddManifest(ctx context.Context, pkg *manifests.PackageManifest) (*[]manifests.PackageManifestLockDependency, error) {
	mgr := defaultRepoLoaderIfNil(r.Loader)

	// Get platform constraints for this scope.
	platform, err := platformsFromConstraints(pkg.Spec.Constraints)
	if err != nil {
		return nil, err
	}

	locks := &[]manifests.PackageManifestLockDependency{}

	scope := solver.Scope[struct{}, buildSD, buildCD]{Data: buildSD{pkg, platform, locks}}

	// Create scope constrainers.
	for _, dep := range pkg.Spec.Dependencies {
		if dep.Image.Range != "" {
			rng, err := semver.NewConstraint(dep.Image.Range)
			if err != nil {
				return nil, err
			}
			scope.Constrainers = append(scope.Constrainers, dependOnFQDNVersion(dep.Image.Package, rng))
		} else {
			scope.Constrainers = append(scope.Constrainers, dependOnFQDNAnyVersion(dep.Image.Package))
		}
	}

	// Fetch all the repositories.
	idx, err := mgr(ctx, pkg.Spec.Repositories)
	if err != nil {
		return nil, err
	}

	// Add all entries of the repositories to the scope.
	for _, entry := range idx.ListAllEntries() {
		// Generate platform constraints for scope.
		allowedPlatformVersions, err := platformsFromConstraints(entry.Data.Constraints)
		if err != nil {
			return nil, err
		}

		// Do for all versions of this entry.
		for _, verStr := range entry.Data.Versions {
			// Create candidate stuff and the candidate itself
			ver, err := semver.NewVersion(strings.TrimPrefix(verStr, "v"))
			if err != nil {
				return nil, err
			}

			newCandidate := solver.Candidate[struct{}, buildSD, buildCD]{
				Data: buildCD{pkg, entry, entry.FQDN(), ver, allowedPlatformVersions},
				Constrainers: []solver.CandidateConstrainer[struct{}, buildSD, buildCD]{
					uniqueInScope,
					containsScopePlatforms,
				},
			}
			chk := func(a solver.Candidate[struct{}, buildSD, buildCD]) bool {
				return a.Data.fqdn == newCandidate.Data.fqdn && a.Data.version.Equal(newCandidate.Data.version)
			}

			// If the version of a package is defined multiple times  report an error.
			if slices.ContainsFunc(scope.Candidates, chk) {
				return nil, fmt.Errorf("%w version %v of package %s exists multiple times in repository entry", ErrRepositoryInconsistent, newCandidate.Data.version, newCandidate.Data.fqdn)
			}
			scope.Candidates = append(scope.Candidates, newCandidate)
		}
	}

	// Sort candidates first by version, then by fqdn.
	chk := func(a, b solver.Candidate[struct{}, buildSD, buildCD]) int {
		if vercmp := b.Data.version.Compare(a.Data.version); vercmp != 0 {
			return vercmp
		}
		return strings.Compare(a.Data.fqdn, b.Data.fqdn)
	}
	slices.SortFunc(scope.Candidates, chk)

	r.inst.Scopes = append(r.inst.Scopes, scope)

	return locks, nil
}

// Solver solves the dependencies for all parameters.
func (r BuildResolver) Solve() (err error) {
	deps, err := solver.Solve(r.inst)
	if err != nil {
		return fmt.Errorf("solving package deps: %w", err)
	}

	for _, dep := range deps {
		depData := dep.CandidateData()
		scopeData := dep.CandidateScopeAccessor().ScopeData()

		for _, originalDep := range depData.forManifest.Spec.Dependencies {
			if originalDep.Image.Package == depData.fqdn {
				pkgDep := manifests.PackageManifestLockDependency{
					Name:    originalDep.Image.Name,
					Image:   depData.entry.Data.Image,
					Digest:  depData.entry.Data.Digest,
					Version: depData.version.String(),
				}
				*scopeData.locks = append(*scopeData.locks, pkgDep)
			}
		}
	}

	return
}
