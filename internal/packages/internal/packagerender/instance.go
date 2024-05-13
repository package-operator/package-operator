package packagerender

import (
	"context"

	"package-operator.run/internal/packages/internal/packagetypes"
)

// Turns a Package and PackageRenderContext into a PackageInstance.
func RenderPackageInstance(
	ctx context.Context, pkg *packagetypes.Package,
	tmplCtx packagetypes.PackageRenderContext,
	pkgValidator packagetypes.PackageValidator,
	objValidator packagetypes.ObjectValidator,
) (*packagetypes.PackageInstance, error) {
	if pkgValidator != nil {
		if err := pkgValidator.ValidatePackage(ctx, pkg); err != nil {
			return nil, err
		}
	}
	if err := RenderTemplates(ctx, pkg, tmplCtx); err != nil {
		return nil, err
	}
	objects, err := RenderObjectsWithFilter(ctx, pkg, tmplCtx, objValidator)
	if err != nil {
		return nil, err
	}
	pkgInst := &packagetypes.PackageInstance{
		Manifest:     pkg.Manifest,
		ManifestLock: pkg.ManifestLock,
		Objects:      objects,
	}
	return pkgInst, nil
}
