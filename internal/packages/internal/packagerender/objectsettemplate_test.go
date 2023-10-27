package packagerender

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages/internal/packageimport"
	"package-operator.run/internal/packages/internal/packagestructure"
	"package-operator.run/internal/packages/internal/packagetypes"
)

var testDataPath = filepath.Join("testdata", "base")

func TestTemplateSpecFromPackage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	rawPkg, err := packageimport.FromFolder(ctx, testDataPath)
	require.NoError(t, err)

	pkg, err := packagestructure.DefaultStructuralLoader.Load(ctx, rawPkg)
	require.NoError(t, err)
	require.NotNil(t, pkg)

	pkgInstance, err := RenderPackageInstance(ctx, pkg, packagetypes.PackageRenderContext{}, nil, nil)
	require.NoError(t, err)

	spec := RenderObjectSetTemplateSpec(pkgInstance)
	require.NotNil(t, spec)
}
