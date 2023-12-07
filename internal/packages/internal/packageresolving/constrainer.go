package packageresolving

import (
	"fmt"

	"github.com/operator-framework/deppy/pkg/deppy"
	"github.com/operator-framework/deppy/pkg/deppy/constraint"
	"pkg.package-operator.run/semver"

	"package-operator.run/internal/solver"
)

// dependOnFQDNVersion creates a constrainer that requires one instance of a candidate identified by the given fqdn.
// Its version must be matched by the given version constraint verConst. The highest available version is chosen.
func dependOnFQDNVersion(fqdn string, verConst semver.Constraint) solver.ScopeConstrainer[struct{}, buildSD, buildCD] {
	return func(s solver.ScopeAccessor[struct{}, buildSD, buildCD]) (cns []deppy.Constraint) {
		// Get solver variable IDs for all packages that fit the fqdn and version constraint.
		selectIDs := []deppy.Identifier{}
		discardedVersions := []string{}

		for _, candidate := range s.ScopeCandidateAccessors() {
			currentVersion := candidate.CandidateData().version
			if fqdn == candidate.CandidateData().fqdn {
				if verConst.Check(currentVersion) {
					// Right package reference and matching version.
					selectIDs = append(selectIDs, candidate.CandidateData().CandidateIdentifier())
				} else {
					// Right package reference but version does not match.
					discardedVersions = append(discardedVersions, currentVersion.String())
				}
			}
		}

		switch {
		case len(selectIDs) != 0:
			// There are candidates.
			cns = append(cns, constraint.Dependency(selectIDs...))
		case len(discardedVersions) != 0:
			// There are candidates for the package reference that were discarded by the version constraint.
			formatter := func(_ deppy.Constraint, subject deppy.Identifier) string {
				return fmt.Sprintf("%s requires package %s with version constraint %s but no available version out of %v satisfies this", subject, fqdn, verConst, discardedVersions)
			}
			cns = append(cns, constraint.NewUserFriendlyConstraint(constraint.Prohibited(), formatter))
		default:
			// There is no canidate for the package reference.
			formatter := func(_ deppy.Constraint, subject deppy.Identifier) string {
				return fmt.Sprintf("%s requires package %s which has no candidates", subject, fqdn)
			}
			cns = append(cns, constraint.NewUserFriendlyConstraint(constraint.Prohibited(), formatter))
		}

		return
	}
}

// dependOnFQDNAnyVersion creates a constrainer that requires one instance of a candidate identified by the given fqdn.
// The highest available version is chosen.
func dependOnFQDNAnyVersion(fqdn string) solver.ScopeConstrainer[struct{}, buildSD, buildCD] {
	return func(s solver.ScopeAccessor[struct{}, buildSD, buildCD]) (cns []deppy.Constraint) {
		// Get solver variable IDs for all packages that fit the fqdn.
		selectIDs := []deppy.Identifier{}
		for _, candidate := range s.ScopeCandidateAccessors() {
			if fqdn == candidate.CandidateData().fqdn {
				selectIDs = append(selectIDs, candidate.CandidateData().CandidateIdentifier())
			}
		}

		if len(selectIDs) != 0 {
			// There are candidates.
			cns = append(cns, constraint.Dependency(selectIDs...))
		} else {
			// There is no canidate for the package reference.
			formatter := func(_ deppy.Constraint, subject deppy.Identifier) string {
				return fmt.Sprintf("%s requires package %s which has no candidates", subject, fqdn)
			}
			cns = append(cns, constraint.NewUserFriendlyConstraint(constraint.Prohibited(), formatter))
		}

		return
	}
}

// uniqueInScope enforces that the candidate it is attached to be unique in the hosting scope.
// Another canidate is considered conflicting when it has the same fqdn. (It does not conflict with itself though).
func uniqueInScope(us solver.CandidateAccessor[struct{}, buildSD, buildCD]) (cns []deppy.Constraint) {
	ourCD := us.CandidateData()

	for _, other := range us.CandidateScopeAccessor().ScopeCandidateAccessors() {
		otherCD := other.CandidateData()
		if ourCD.fqdn == otherCD.fqdn && !ourCD.version.Equal(otherCD.version) {
			// Other matches our package and is not us.
			cns = append(cns, constraint.Conflict(otherCD.CandidateIdentifier()))
		}
	}

	return
}

// containsScopePlatforms enforces that the platform constraints of the attached candidate allow for the full range
// of the platform constraints of the scope.
func containsScopePlatforms(us solver.CandidateAccessor[struct{}, buildSD, buildCD]) (cns []deppy.Constraint) {
	// All scope platforms must be supported.
	for scopeName, scopeConstraint := range us.CandidateScopeAccessor().ScopeData().platforms {
		ourPlatform, oursOK := us.CandidateData().platforms[scopeName]
		switch {
		case !oursOK:
			// Candidate does not support scope platform at all.
			formatter := func(_ deppy.Constraint, subject deppy.Identifier) string {
				return fmt.Sprintf("support for platform %q is required but %s does not support it", scopeName, subject)
			}
			cns = append(cns, constraint.NewUserFriendlyConstraint(constraint.Prohibited(), formatter))
		case !ourPlatform.Contains(scopeConstraint):
			// Candidate does not support the full range of the scope platform.
			formatter := func(_ deppy.Constraint, subject deppy.Identifier) string {
				return fmt.Sprintf("support for platform %q within range %q is required but %s supports only %q ", subject, scopeName, ourPlatform.String(), scopeConstraint)
			}
			cns = append(cns, constraint.NewUserFriendlyConstraint(constraint.Prohibited(), formatter))
		}
	}

	return
}
