package packageexport

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/packages/packagecontent"
)

func Image(files packagecontent.Files) (v1.Image, error) {
	configFile := &v1.ConfigFile{Architecture: "amd64", OS: "linux", Config: v1.Config{}, RootFS: v1.RootFS{Type: "layers"}}
	image, err := mutate.ConfigFile(empty.Image, configFile)
	if err != nil {
		return nil, err
	}

	subFiles := packagecontent.Files{}
	for k, v := range files {
		subFiles[filepath.Join(packages.ImageFilePrefixPath, k)] = v
	}

	layer, err := crane.Layer(subFiles)
	if err != nil {
		return nil, err
	}

	image, err = mutate.AppendLayers(image, layer)
	if err != nil {
		return nil, fmt.Errorf("create image from layer: %w", err)
	}

	image, err = mutate.Canonical(image)
	if err != nil {
		return nil, err
	}

	return image, nil
}

func PushedImage(ctx context.Context, references []string, files packagecontent.Files, opts ...crane.Option) error {
	image, err := Image(files)
	if err != nil {
		return err
	}

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
