package resolvebuild

import (
	"fmt"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages"
	"package-operator.run/internal/packages/resolving/solver"

	"pkg.package-operator.run/semver"
)

type candidateData struct {
	forPkg *manifests.PackageManifest
	entry  packages.Entry

	fqdn      string
	version   semver.Version
	platforms map[string]semver.Constraint
}

func (c candidateData) CandidateIdentifier() solver.Identifier {
	return solver.Identifier(fmt.Sprintf("dependency %s@%s for %s", c.entry.Data.Name, c.version.String(), c.forPkg.Name))
}

type scopeData struct {
	pkg       *manifests.PackageManifest
	platforms map[string]semver.Constraint
}

func (s scopeData) ScopeIdentifier() solver.Identifier {
	return solver.Identifier(fmt.Sprintf("package %s", s.pkg.Name))
}
