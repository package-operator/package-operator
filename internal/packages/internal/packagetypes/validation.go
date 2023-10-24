package packagetypes

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"package-operator.run/internal/apis/manifests"
)

// PackageValidator knows how to validate Packages.
type PackageValidator interface {
	ValidatePackage(ctx context.Context, pkg *Package) error
}

// ObjectValidator knows how to validate objects within a Package.
type ObjectValidator interface {
	ValidateObjects(
		ctx context.Context,
		manifest *manifests.PackageManifest,
		objects map[string][]unstructured.Unstructured,
	) error
}
