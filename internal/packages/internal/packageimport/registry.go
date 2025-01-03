package packageimport

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/packages/internal/packageimport/kubekeychain"
	"package-operator.run/internal/packages/internal/packagetypes"
)

// Imports a RawPackage from a container image registry,
// while supplying pull credentials which are dynamically discovered from the ServiceAccount PKO is running under.
func FromRegistryInCluster(
	ctx context.Context, uncachedClient client.Client, serviceAccount types.NamespacedName,
	ref string, opts ...crane.Option,
) (*packagetypes.RawPackage, error) {
	chain, err := kubekeychain.FromServiceAccountPullSecrets(ctx, uncachedClient, serviceAccount)
	if err != nil {
		return nil, fmt.Errorf("creating keychain: %w", err)
	}
	opts = append(opts, crane.WithAuthFromKeychain(chain))
	return FromRegistry(ctx, ref, opts...)
}

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
