package packagestructure

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages/internal/packageimport"
	"package-operator.run/internal/packages/internal/packagetypes"
)

func TestStructuralLoader_LoadComponent(t *testing.T) {
	t.Parallel()
	sl := NewStructuralLoader(scheme)

	t.Run("components-disabled", func(t *testing.T) {
		t.Parallel()
		ctx := logr.NewContext(t.Context(), testr.New(t))
		rawPkg, err := packageimport.FromFolder(ctx, "testdata/multi-component/components-disabled")
		require.NoError(t, err)

		_, err = sl.LoadComponent(ctx, rawPkg, "frontend")
		require.EqualError(t, err, packagetypes.ViolationError{
			Reason: packagetypes.ViolationReasonComponentsNotEnabled,
		}.Error())
	})

	t.Run("root", func(t *testing.T) {
		t.Parallel()
		ctx := logr.NewContext(t.Context(), testr.New(t))
		rawPkg, err := packageimport.FromFolder(ctx, "testdata/multi-component/components-enabled/valid")
		require.NoError(t, err)

		pkg, err := sl.LoadComponent(ctx, rawPkg, "")
		require.NoError(t, err)

		assert.NotNil(t, pkg)
		assert.NotNil(t, pkg.Manifest, "Manifest must exist")
		if assert.Len(t, pkg.Files, 1) {
			assert.NotEmpty(t, pkg.Files["ConfigMap.yaml"])
		}
		assert.Empty(t, pkg.Components)
	})

	t.Run("subcomponent", func(t *testing.T) {
		t.Parallel()
		ctx := logr.NewContext(t.Context(), testr.New(t))
		rawPkg, err := packageimport.FromFolder(ctx, "testdata/multi-component/components-enabled/valid")
		require.NoError(t, err)

		pkg, err := sl.LoadComponent(ctx, rawPkg, "frontend")
		require.NoError(t, err)

		assert.NotNil(t, pkg)
		assert.NotNil(t, pkg.Manifest, "Manifest must exist")
		if assert.Len(t, pkg.Files, 1) {
			assert.NotEmpty(t, pkg.Files["Deployment.yaml"])
		}
		assert.Empty(t, pkg.Components)
	})

	t.Run("non-existing-subcomponent", func(t *testing.T) {
		t.Parallel()
		ctx := logr.NewContext(t.Context(), testr.New(t))
		rawPkg, err := packageimport.FromFolder(ctx, "testdata/multi-component/components-enabled/valid")
		require.NoError(t, err)

		nonExistingComponent := "foobar"

		_, err = sl.LoadComponent(ctx, rawPkg, nonExistingComponent)
		require.EqualError(t, err, packagetypes.ViolationError{
			Reason:    packagetypes.ViolationReasonComponentNotFound,
			Component: nonExistingComponent,
		}.Error())
	})
}

func TestStructuralLoader_Load(t *testing.T) {
	t.Parallel()
	sl := NewStructuralLoader(scheme)

	t.Run("base", func(t *testing.T) {
		t.Parallel()
		ctx := logr.NewContext(t.Context(), testr.New(t))
		rawPkg, err := packageimport.FromFolder(ctx, "testdata/base")
		require.NoError(t, err)

		pkg, err := sl.Load(ctx, rawPkg)
		require.NoError(t, err)

		assert.NotNil(t, pkg)
		assert.NotNil(t, pkg.Manifest, "Manifest must exist")
		assert.Nil(t, pkg.ManifestLock, "no lockfile")
		if assert.Len(t, pkg.Files, 2) {
			assert.NotEmpty(t, pkg.Files["Containerfile"])
			assert.NotEmpty(t, pkg.Files["some-statefulset.yaml"])
		}
		assert.Empty(t, pkg.Components)
	})

	t.Run("components-disabled", func(t *testing.T) {
		t.Parallel()
		ctx := logr.NewContext(t.Context(), testr.New(t))
		rawPkg, err := packageimport.FromFolder(ctx, "testdata/multi-component/components-disabled")
		require.NoError(t, err)

		pkg, err := sl.Load(ctx, rawPkg)
		require.NoError(t, err)

		assert.NotNil(t, pkg)
		assert.NotNil(t, pkg.Manifest, "Manifest must exist")
		assert.NotNil(t, pkg.ManifestLock, "ManifestLock must exist")
		if assert.Len(t, pkg.Files, 3) {
			assert.NotEmpty(t, pkg.Files["ConfigMap.yaml"])
			assert.NotEmpty(t, pkg.Files["components/backend/Deployment.yaml"])
			assert.NotEmpty(t, pkg.Files["components/frontend/Deployment.yaml"])
		}
		assert.Empty(t, pkg.Components)
	})

	t.Run("components-enabled/valid", func(t *testing.T) {
		t.Parallel()
		ctx := logr.NewContext(t.Context(), testr.New(t))
		rawPkg, err := packageimport.FromFolder(ctx, "testdata/multi-component/components-enabled/valid")
		require.NoError(t, err)

		pkg, err := sl.Load(ctx, rawPkg)
		require.NoError(t, err)

		assert.NotNil(t, pkg)
		assert.NotNil(t, pkg.Manifest, "Manifest must exist")
		if assert.Len(t, pkg.Files, 1) {
			assert.NotEmpty(t, pkg.Files["ConfigMap.yaml"])
		}
		assert.Len(t, pkg.Components, 2)

		for _, comp := range pkg.Components {
			assert.NotNil(t, comp.Manifest, "Manifest must exist")
			assert.Empty(t, comp.Components)
			if assert.Len(t, comp.Files, 1) {
				assert.NotEmpty(t, comp.Files["Deployment.yaml"])
			}
		}
	})

	t.Run("components-enabled/nested", func(t *testing.T) {
		t.Parallel()
		ctx := logr.NewContext(t.Context(), testr.New(t))
		rawPkg, err := packageimport.FromFolder(ctx, "testdata/multi-component/components-enabled/nested-components")
		require.NoError(t, err)

		_, err = sl.Load(ctx, rawPkg)
		require.EqualError(t, err, packagetypes.ViolationError{
			Reason:    packagetypes.ViolationReasonNestedMultiComponentPkg,
			Component: "backend",
		}.Error())
	})

	t.Run("components-enabled/invalid-files-in-components-dir", func(t *testing.T) {
		t.Parallel()
		ctx := logr.NewContext(t.Context(), testr.New(t))
		rawPkg, err := packageimport.FromFolder(
			ctx,
			"testdata/multi-component/components-enabled/invalid-files-in-components-dir",
		)
		require.NoError(t, err)

		_, err = sl.Load(ctx, rawPkg)
		require.EqualError(t, err, packagetypes.ViolationError{
			Reason: packagetypes.ViolationReasonInvalidFileInComponentsDir,
			Path:   "components/foobar",
		}.Error())
	})

	t.Run("duplicated-manifest", func(t *testing.T) {
		t.Parallel()
		ctx := logr.NewContext(t.Context(), testr.New(t))
		rawPkg, err := packageimport.FromFolder(ctx, "testdata/duplicated-manifest")
		require.NoError(t, err)

		_, err = sl.Load(ctx, rawPkg)
		require.EqualError(t, err, packagetypes.ViolationError{
			Reason: packagetypes.ViolationReasonPackageManifestDuplicated,
		}.Error())
	})

	t.Run("duplicated-manifest-lock", func(t *testing.T) {
		t.Parallel()
		ctx := logr.NewContext(t.Context(), testr.New(t))
		rawPkg, err := packageimport.FromFolder(ctx, "testdata/duplicated-manifest-lock")
		require.NoError(t, err)

		_, err = sl.Load(ctx, rawPkg)
		require.EqualError(t, err, packagetypes.ViolationError{
			Reason: packagetypes.ViolationReasonPackageManifestLockDuplicated,
		}.Error())
	})

	t.Run("missing-manifest", func(t *testing.T) {
		t.Parallel()
		ctx := logr.NewContext(t.Context(), testr.New(t))
		rawPkg, err := packageimport.FromFolder(ctx, "testdata/missing-manifest")
		require.NoError(t, err)

		_, err = sl.Load(ctx, rawPkg)
		require.EqualError(t, err, packagetypes.ErrManifestNotFound.Error())
	})
}
