package export

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func TarToDisk(dst string, tags []string, image v1.Image) error {
	m := map[string]v1.Image{}
	for _, tag := range tags {
		m[tag] = image
	}
	if err := crane.MultiSave(m, dst); err != nil {
		return fmt.Errorf("dump to %s: %w", dst, err)
	}

	return nil
}

func Push(ctx context.Context, references []string, image v1.Image, opts ...crane.Option) error {
	opts = append(opts, crane.WithContext(ctx))
	verboseLogger := logr.FromContextOrDiscard(ctx).V(1)
	for _, ref := range references {
		verboseLogger.Info("pushing image", "reference", ref)
		err := crane.Push(image, ref, opts...)
		if err != nil {
			return fmt.Errorf("push: %w", err)
		}
	}

	return nil
}
