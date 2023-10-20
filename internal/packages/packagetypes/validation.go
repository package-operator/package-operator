package packagetypes

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"package-operator.run/internal/apis/manifests"
)

type PackageValidator interface {
	ValidatePackage(ctx context.Context, pkg *Package) error
}

type ObjectValidator interface {
	ValidateObjects(
		ctx context.Context,
		manifest *manifests.PackageManifest,
		objects map[string][]unstructured.Unstructured,
	) error
}
