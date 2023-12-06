package resolvebuild

import (
	"fmt"

	"github.com/operator-framework/deppy/pkg/deppy"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages"

	"pkg.package-operator.run/semver"
)

type candidateData struct {
	forPkg *manifests.PackageManifest
	entry  packages.Entry

	fqdn      string
	version   semver.Version
	platforms map[string]semver.Constraint
}

func (c candidateData) CandidateIdentifier() deppy.Identifier {
	return deppy.Identifier(fmt.Sprintf("dependency %s@%s for package %s", c.entry.Data.Name, c.version.String(), c.forPkg.Name))
}

type scopeData struct {
	pkg       *manifests.PackageManifest
	platforms map[string]semver.Constraint
}

func (s scopeData) ScopeIdentifier() deppy.Identifier {
	return deppy.Identifier(fmt.Sprintf("package %s", s.pkg.Name))
}
