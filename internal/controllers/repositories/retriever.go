package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"

	"package-operator.run/internal/packages"
)

const (
	RepoRetrieverPullErrMsg = "repository pull error"
	RepoRetrieverLoadErrMsg = "repository load error"
)

var (
	ErrRepoRetrieverPull = errors.New(RepoRetrieverPullErrMsg)
	ErrRepoRetrieverLoad = errors.New(RepoRetrieverLoadErrMsg)
)

type RepoRetriever interface {
	Retrieve(ctx context.Context, image string) (*packages.RepositoryIndex, error)
}

type CraneRepoRetriever struct{}

func (r *CraneRepoRetriever) Retrieve(ctx context.Context, ref string) (*packages.RepositoryIndex, error) {
	image, err := crane.Pull(ref)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", RepoRetrieverPullErrMsg, err)
	}
	idx, err := packages.LoadRepositoryFromOCI(ctx, image)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", RepoRetrieverLoadErrMsg, err)
	}
	return idx, nil
}
