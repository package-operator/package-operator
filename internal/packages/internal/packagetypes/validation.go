package packagetypes

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"package-operator.run/internal/apis/manifests"
)

// PackageValidator knows how to validate Packages.
type PackageValidator interface {
	ValidatePackage(ctx context.Context, pkg *Package) error
}

func ValidateEachComponent(
	ctx context.Context, pkg *Package,
	validateFn func(context.Context, *Package, bool) error,
) error {
	for _, component := range pkg.Components {
		componentName := component.Manifest.Name
		if err := validateFn(ctx, &component, true); err != nil {
			return fmt.Errorf("component \"%s\": %w", componentName, err)
		}
	}

	return validateFn(ctx, pkg, false)
}

// ObjectValidator knows how to validate objects within a Package.
type ObjectValidator interface {
	ValidateObjects(
		ctx context.Context,
		manifest *manifests.PackageManifest,
		objects map[string][]unstructured.Unstructured,
	) error
}
