package packagevalidation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
)

func TestPackageScopeValidator(t *testing.T) {
	t.Parallel()

	scopeV := PackageScopeValidator(manifests.PackageManifestScopeCluster)

	ctx := context.Background()
	manifest := &manifests.PackageManifest{
		Spec: manifests.PackageManifestSpec{
			Scopes: []manifests.PackageManifestScope{manifests.PackageManifestScopeNamespaced},
		},
	}
	err := scopeV.ValidatePackage(ctx, &packagetypes.Package{
		Manifest: manifest,
	})
	require.EqualError(t, err, "Package unsupported scope in manifest.yaml")
}
