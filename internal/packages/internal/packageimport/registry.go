package packageimport

import (
	"context"

	"github.com/google/go-containerregistry/pkg/crane"

	"package-operator.run/internal/packages/internal/packagetypes"
)

// Imports a RawPackage from a container image registry.
func FromRegistry(
	ctx context.Context, ref string, opts ...crane.Option,
) (*packagetypes.RawPackage, error) {
	img, err := crane.Pull(ref, opts...)
	if err != nil {
		return nil, err
	}
	return FromOCI(ctx, img)
}
