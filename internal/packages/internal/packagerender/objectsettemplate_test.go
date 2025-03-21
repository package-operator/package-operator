package packagerender

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/packages/internal/packageimport"
	"package-operator.run/internal/packages/internal/packagestructure"
	"package-operator.run/internal/packages/internal/packagetypes"
)

var testDataPath = filepath.Join("testdata", "base")

func TestTemplateSpecFromPackage(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	rawPkg, err := packageimport.FromFolder(ctx, testDataPath)
	require.NoError(t, err)

	pkg, err := packagestructure.DefaultStructuralLoader.Load(ctx, rawPkg)
	require.NoError(t, err)
	require.NotNil(t, pkg)

	pkgInstance, err := RenderPackageInstance(ctx, pkg, packagetypes.PackageRenderContext{}, nil, nil)
	require.NoError(t, err)

	spec := RenderObjectSetTemplateSpec(pkgInstance)
	require.NotNil(t, spec)

	require.Len(t, spec.Phases, 1)

	// assert objects are in the right order
	assert.Equal(t, []string{
		"/v1, Kind=ConfigMap /zzz",
		"apps/v1, Kind=StatefulSet /some-stateful-set-1",
		"/v1, Kind=ConfigMap /t4",
		"/v1, Kind=ConfigMap /abc",
	}, objectsToKindNameString(spec.Phases[0].Objects))
}

func objectsToKindNameString(objects []v1alpha1.ObjectSetObject) []string {
	out := make([]string, len(objects))
	for i, obj := range objects {
		out[i] = fmt.Sprintf("%s %s/%s",
			obj.Object.GroupVersionKind(),
			obj.Object.GetNamespace(),
			obj.Object.GetName())
	}
	return out
}
