package bootstrap

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"package-operator.run/internal/packages"
)

type packageObjectLoader interface {
	FromPkg(
		ctx context.Context, rawPkg *packages.RawPackage,
	) ([]unstructured.Unstructured, error)
}

type bootstrapperPullImageFn func(
	ctx context.Context, image string) (*packages.RawPackage, error)

type packageObjectLoad struct{}

func (pol *packageObjectLoad) FromPkg(
	ctx context.Context, rawPkg *packages.RawPackage,
) ([]unstructured.Unstructured, error) {
	pkg, err := packages.DefaultStructuralLoader.Load(ctx, rawPkg)
	if err != nil {
		return nil, err
	}
	return packages.RenderObjectsWithFilter(ctx, pkg, packages.PackageRenderContext{}, nil)
}
