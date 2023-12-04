package resolvebuild

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages"
)

type RepoLoader func(context.Context, []manifests.PackageManifestRepository) (*packages.MultiRepositoryIndex, error)

func defaultRepoLoaderIfNil(r RepoLoader) RepoLoader {
	if r == nil {
		return loadRepo
	}

	return r
}

func loadRepo(ctx context.Context, repos []manifests.PackageManifestRepository) (*packages.MultiRepositoryIndex, error) {
	idx := packages.NewMultiRepositoryIndex()
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
