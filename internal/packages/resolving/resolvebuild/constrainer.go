package resolvebuild

import (
	"errors"
	"fmt"

	"github.com/operator-framework/deppy/pkg/deppy"
	"github.com/operator-framework/deppy/pkg/deppy/constraint"
	"pkg.package-operator.run/semver"

	"package-operator.run/internal/packages/resolving/solver"
)

var ErrRepositoryInconsistent = errors.New("package repository inconsistent")

func dependOnConstrained(fqdn string, verConst semver.Constraint) solver.ScopeConstrainer[struct{}, scopeData, candidateData] {
	return func(s solver.ScopeAccessor[struct{}, scopeData, candidateData]) []deppy.Constraint {
		// Get solver variable IDs for all packages that fit the version constraint.
		selectIDs := []deppy.Identifier{}
		discardedVersions := []string{}
		for _, candidate := range s.ScopeCandidateAccessors() {
			currentName := candidate.CandidateData().fqdn
			currentVersion := candidate.CandidateData().version
			switch {
			case fqdn != currentName:
				// Wrong package reference.
				continue
			case verConst.Check(currentVersion):
				// Right package reference and matching version.
				selectIDs = append(selectIDs, candidate.CandidateData().CandidateIdentifier())
			default:
				// Right package reference but version does not match.
				discardedVersions = append(discardedVersions, currentVersion.String())
			}
		}

		switch {
		case len(selectIDs) != 0:
			// There are candidates.
			return []deppy.Constraint{constraint.Dependency(selectIDs...)}
		case len(discardedVersions) != 0:
			// There are candidates for the package reference that were discarded by the version constraint.
			formatter := func(_ deppy.Constraint, subject deppy.Identifier) string {
				return fmt.Sprintf("%s requires package %s with version constraint %s but no available version out of %v satisfies this", subject, fqdn, verConst, discardedVersions)
			}
			return []deppy.Constraint{constraint.NewUserFriendlyConstraint(constraint.Prohibited(), formatter)}
		default:
			// There is no canidate for the package reference.
			formatter := func(_ deppy.Constraint, subject deppy.Identifier) string {
				return fmt.Sprintf("%s requires package %s which has no candidates", subject, fqdn)
			}
			return []deppy.Constraint{constraint.NewUserFriendlyConstraint(constraint.Dependency(), formatter)}
		}
	}
}

func dependOnUnconstrained(fqdn string) solver.ScopeConstrainer[struct{}, scopeData, candidateData] {
	return func(s solver.ScopeAccessor[struct{}, scopeData, candidateData]) []deppy.Constraint {
		// Get solver variable IDs for all packages that fit the version constraint.
		selectIDs := []deppy.Identifier{}
		for _, candidate := range s.ScopeCandidateAccessors() {
			currentName := candidate.CandidateData().fqdn
			switch {
			case fqdn != currentName:
				// Wrong package reference.
				continue
			default:
				selectIDs = append(selectIDs, candidate.CandidateData().CandidateIdentifier())
			}
		}

		switch {
		case len(selectIDs) != 0:
			// There are candidates.
			return []deppy.Constraint{constraint.Dependency(selectIDs...)}
		default:
			// There is no canidate for the package reference.
			formatter := func(_ deppy.Constraint, subject deppy.Identifier) string {
				return fmt.Sprintf("%s requires package %s which has no candidates", subject, fqdn)
			}
			return []deppy.Constraint{constraint.NewUserFriendlyConstraint(constraint.Dependency(), formatter)}
		}
	}
}

func uniqueInScope(us solver.CandidateAccessor[struct{}, scopeData, candidateData]) []deppy.Constraint {
	constraints := []deppy.Constraint{}

	ourReference := us.CandidateData().fqdn
	ourVersion := us.CandidateData().version

	for _, other := range us.CandidateScopeAccessor().ScopeCandidateAccessors() {
		otherReference := other.CandidateData().fqdn
		otherVersion := other.CandidateData().version

		switch {
		case ourReference == otherReference && ourVersion.Equal(otherVersion):
			// Other is us, we do not want to conflict with ourselves.
		case ourReference == otherReference:
			// Other matches our package.
			constraints = append(constraints, constraint.Conflict(other.CandidateData().CandidateIdentifier()))
		default:
			// We do not care about the other package.
		}
	}

	return constraints
}

func platformMatches(us solver.CandidateAccessor[struct{}, scopeData, candidateData]) []deppy.Constraint {
	scopePlatforms := us.CandidateScopeAccessor().ScopeData().platforms
	ourPlatforms := us.CandidateData().platforms

	for scopeName, scopeConstraint := range scopePlatforms {
		ourPlatform, oursOK := ourPlatforms[scopeName]
		switch {
		case !oursOK:
			formatter := func(_ deppy.Constraint, subject deppy.Identifier) string {
				return fmt.Sprintf("support for platform %q is required but %s does not support it", scopeName, subject)
			}
			return []deppy.Constraint{constraint.NewUserFriendlyConstraint(constraint.Prohibited(), formatter)}
		case !ourPlatform.Contains(scopeConstraint):
			formatter := func(_ deppy.Constraint, subject deppy.Identifier) string {
				return fmt.Sprintf("support for platform %q within range %q is required but %s supports only %q ", subject, scopeName, ourPlatform.String(), scopeConstraint)
			}
			return []deppy.Constraint{constraint.NewUserFriendlyConstraint(constraint.Prohibited(), formatter)}
		}
	}

	return nil
}
