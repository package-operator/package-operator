package packageresolving

import (
	"fmt"
	"slices"

	"package-operator.run/internal/apis/manifests"

	"pkg.package-operator.run/semver"
)

// platformsFromConstraints parses a set of PackageManifestConstraints. It returns a map which has
// platform names as keys and for each platform name a constrains the defines which version of this
// platform are supported.
func platformsFromConstraints(constraints []manifests.PackageManifestConstraint) (map[string]semver.Constraint, error) {
	// Each PackageManifest constraint is parsed into a slice of platform names that it supports.
	platformTypeAllowSets := [][]string{}
	for idx, c := range constraints {
		var constraintPlatformTypes []string
		for _, constraintPlatformType := range c.Platform {
			if constraintPlatformType == "" {
				return nil, fmt.Errorf("%w:  constraint #%d has empty platform type in allow list", ErrRepositoryInconsistent, idx)
			}
			constraintPlatformTypes = append(constraintPlatformTypes, string(constraintPlatformType))
		}
		if len(constraintPlatformTypes) != 0 {
			platformTypeAllowSets = append(platformTypeAllowSets, constraintPlatformTypes)
		}
	}

	// flatten platformTypeAllowSets into a set of unique platform names that are known.
	knownPlatformTypes := []string{}
	for _, a := range platformTypeAllowSets {
		knownPlatformTypes = append(knownPlatformTypes, a...)
	}
	slices.Sort(knownPlatformTypes)
	knownPlatformTypes = slices.Compact(knownPlatformTypes)

	// This is the returned result
	allowedPlatformVersions := map[string]semver.Constraint{}

	// all platforms that are known here get allowed in any version.
	rangeWild := semver.MustNewConstraint("x-x")
	for _, knownPlatformType := range knownPlatformTypes {
		knownPlatformTypeInAllAllowSets := true
		for _, platformTypeAllowSet := range platformTypeAllowSets {
			knownPlatformTypeInAllAllowSets = slices.Contains(platformTypeAllowSet, knownPlatformType)
			if !knownPlatformTypeInAllAllowSets {
				break
			}
		}
		if knownPlatformTypeInAllAllowSets {
			allowedPlatformVersions[knownPlatformType] = rangeWild
		}
	}

	// If a constraint specifies a version constraint we replace the wild allow with the specific version.
	for idx, c := range constraints {
		if c.PlatformVersion != nil {
			name := string(c.PlatformVersion.Name)
			if name == "" {
				return nil, fmt.Errorf(
					"%w: constraint #%d has empty platform type for version range", ErrRepositoryInconsistent, idx,
				)
			}

			con, err := semver.NewConstraint(c.PlatformVersion.Range)
			if err != nil {
				return nil, fmt.Errorf("%w: constraint #%d has invalid range: %w", ErrRepositoryInconsistent, idx, err)
			}

			allowedPlatformVersions[name] = con
		}
	}

	return allowedPlatformVersions, nil
}
