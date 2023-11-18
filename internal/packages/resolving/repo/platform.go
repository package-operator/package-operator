package repo

import (
	"fmt"

	"github.com/operator-framework/deppy/pkg/deppy"
	"github.com/operator-framework/deppy/pkg/deppy/constraint"
	"golang.org/x/exp/maps"

	"package-operator.run/internal/packages/resolving/solver"
)

// PlatformTypeVersionAccessor allows accessing platform information.
type PlatformTypeVersionAccessor interface {
	solver.InstallationData
	// PlatformType returns the type of the platform.
	PlatformType() string
	// PlatformVersion returns the version of the platform.
	PlatformVersion() solver.Version
}

// RequireInstallationPlatformVersionToBeOneOf requires that the platform running the [Candidate] is of one of the [PlatformType] denoted by the keys
// in the parameter and must match the version constraint of its value.
func RequireInstallationPlatformVersionToBeOneOf[IM PlatformTypeVersionAccessor, SM solver.ScopeData, CM solver.CandidateData](oneOf map[string]solver.VersionRange) solver.CandidateConstrainer[IM, SM, CM] {
	return func(c solver.CandidateAccessor[IM, SM, CM]) []solver.Constraint {
		installationMetadata := c.CandidateScopeAccessor().ScopeInstallationAccessor().InstallationData()
		platformType := installationMetadata.PlatformType()
		platformVersion := installationMetadata.PlatformVersion()

		versionConstraint, platformPresent := oneOf[platformType]
		if !platformPresent {
			formatter := func(_ deppy.Constraint, subject deppy.Identifier) string {
				return fmt.Sprintf("%s only allows platform types %v but we have %s", subject, maps.Keys(oneOf), platformType)
			}
			return []solver.Constraint{constraint.NewUserFriendlyConstraint(constraint.Prohibited(), formatter)}
		}

		if !versionConstraint.Check(&platformVersion) {
			formatter := func(_ deppy.Constraint, subject deppy.Identifier) string {
				return fmt.Sprintf("%s allows platform %s only in version range %s but we have %s", subject, platformType, versionConstraint.String(), platformVersion)
			}
			return []solver.Constraint{constraint.NewUserFriendlyConstraint(constraint.Prohibited(), formatter)}
		}

		return nil
	}
}
