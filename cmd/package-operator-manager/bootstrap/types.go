package bootstrap

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"package-operator.run/internal/packages/packagerender"
	"package-operator.run/internal/packages/packagestructure"
	"package-operator.run/internal/packages/packagetypes"
)

type packageObjectLoader interface {
	FromPkg(
		ctx context.Context, rawPkg *packagetypes.RawPackage,
	) ([]unstructured.Unstructured, error)
}

type bootstrapperPullImageFn func(
	ctx context.Context, image string) (*packagetypes.RawPackage, error)

type packageObjectLoad struct{}

func (pol *packageObjectLoad) FromPkg(
	ctx context.Context, rawPkg *packagetypes.RawPackage,
) ([]unstructured.Unstructured, error) {
	pkg, err := packagestructure.DefaultStructuralLoader.Load(ctx, rawPkg)
	if err != nil {
		return nil, err
	}
	return packagerender.RenderObjects(ctx, pkg, packagetypes.PackageRenderContext{}, nil)
}
