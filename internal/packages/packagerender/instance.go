package packagerender

import (
	"context"

	"package-operator.run/internal/packages/packagetypes"
)

func RenderPackageInstance(
	ctx context.Context, pkg *packagetypes.Package,
	tmplCtx packagetypes.PackageRenderContext,
	pkgValidator packagetypes.PackageValidator,
	objValidator packagetypes.ObjectValidator,
) (*packagetypes.PackageInstance, error) {
	if err := pkgValidator.ValidatePackage(ctx, pkg); err != nil {
		return nil, err
	}
	if err := RenderTemplates(ctx, pkg, tmplCtx); err != nil {
		return nil, err
	}
	objects, err := RenderObjects(ctx, pkg, tmplCtx, objValidator)
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
