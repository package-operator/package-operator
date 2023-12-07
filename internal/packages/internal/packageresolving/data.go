package packageresolving

import (
	"fmt"

	"github.com/operator-framework/deppy/pkg/deppy"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagerepository"

	"pkg.package-operator.run/semver"
)

// buildCD describes the candidate of a scope.
// It represents the package dependency that is installed for a component.
type buildCD struct {
	// Manifest of the component that requested this candidate.
	forManifest *manifests.PackageManifest
	// repository entry of this canddate.
	entry packagerepository.Entry
	// fqdn of this candidate.
	fqdn string
	// version of this candidate.
	version semver.Version
	// supported platforms by this candidate.
	platforms map[string]semver.Constraint
}

func (c buildCD) CandidateIdentifier() deppy.Identifier {
	return deppy.Identifier(fmt.Sprintf("dependency %s@%s for package %s", c.entry.Data.Name, c.version.String(), c.forManifest.Name))
}

type buildSD struct {
	// Manifest of this scope - a package that has dependencies.
	manifests *manifests.PackageManifest
	// supported platforms by this scope.
	platforms map[string]semver.Constraint
	// destination for generated locks.
	locks *[]manifests.PackageManifestLockDependency
}

func (s buildSD) ScopeIdentifier() deppy.Identifier {
	return deppy.Identifier(fmt.Sprintf("package %s", s.manifests.Name))
}
