package resolvebuild

import (
	"fmt"
	"slices"

	"package-operator.run/internal/apis/manifests"

	"pkg.package-operator.run/semver"
)

func platformsFromConstraints(constraints []manifests.PackageManifestConstraint) (map[string]semver.Constraint, error) {
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

	knownPlatformTypes := []string{}
	for _, a := range platformTypeAllowSets {
		knownPlatformTypes = append(knownPlatformTypes, a...)
	}
	slices.Sort(knownPlatformTypes)
	knownPlatformTypes = slices.Compact(knownPlatformTypes)

	rangeWild, err := semver.NewConstraint("x-x")
	if err != nil {
		panic(err)
	}
	allowedPlatformVersions := map[string]semver.Constraint{}
	for _, knownPlatformType := range knownPlatformTypes {
		knownPlatformTypeInAllAllowSets := true
		for _, platformTypeAllowSet := range platformTypeAllowSets {
			if knownPlatformTypeInAllAllowSets = slices.Contains(platformTypeAllowSet, knownPlatformType); !knownPlatformTypeInAllAllowSets {
				break
			}
		}
		if knownPlatformTypeInAllAllowSets {
			allowedPlatformVersions[knownPlatformType] = rangeWild
		}
	}

	for idx, c := range constraints {
		if c.PlatformVersion != nil {
			name := string(c.PlatformVersion.Name)
			if name == "" {
				return nil, fmt.Errorf("%w: constraint #%d has empty platform type for version range", ErrRepositoryInconsistent, idx)
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
