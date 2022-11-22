package export

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func ComressedTarToDisk(dst string, references []name.Reference, image v1.Image) error {
	imagesByRef := map[name.Reference]v1.Image{}
	for _, ref := range references {
		imagesByRef[ref] = image
	}

	if err := tarball.MultiRefWriteToFile(dst, imagesByRef); err != nil {
		return fmt.Errorf("dump to %s: %w", dst, err)
	}

	return nil
}

func Push(ctx context.Context, references []name.Reference, image v1.Image, opts ...crane.Option) error {
	opts = append(opts, crane.WithContext(ctx))
	verboseLogger := logr.FromContextOrDiscard(ctx).V(1)
	for _, ref := range references {
		verboseLogger.Info("pushing image", "reference", ref)
		err := crane.Push(image, ref.String(), opts...)
		if err != nil {
			return fmt.Errorf("push: %w", err)
		}
	}

	return nil
}
