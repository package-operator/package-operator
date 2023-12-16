package packageresolving

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagerepository"
)

// RepoLoader is an interface for somthing that takes a set PackageManifestRepositories and loads
// thoses into a MultiRepositoryIndex.
type RepoLoader func(
	context.Context, []manifests.PackageManifestRepository,
) (*packagerepository.MultiRepositoryIndex, error)

// defaultRepoLoaderIfNil passes through r if it is not nil, elsewise it returns [loadRepo].
func defaultRepoLoaderIfNil(r RepoLoader) RepoLoader {
	if r == nil {
		return loadRepo
	}

	return r
}

// loadRepo pulls the given PackageManifestRepositories into a MultiRepositoryIndex.
func loadRepo(
	ctx context.Context, repos []manifests.PackageManifestRepository,
) (*packagerepository.MultiRepositoryIndex, error) {
	idx := packagerepository.NewMultiRepositoryIndex()
	for _, r := range repos {
		if r.File != "" {
			if err := idx.LoadRepositoryFromFile(ctx, r.File); err != nil {
				return idx, err
			}
		}

		if r.Image != "" {
			image, err := crane.Pull(r.Image)
			if err != nil {
				return idx, fmt.Errorf("pull repository image: %w", err)
			}

			if err := idx.LoadRepositoryFromOCI(ctx, image); err != nil {
				return idx, err
			}
		}
	}

	return idx, nil
}
