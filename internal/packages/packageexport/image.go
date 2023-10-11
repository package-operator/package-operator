package packageexport

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"package-operator.run/internal/packages"
	"package-operator.run/internal/packages/packagecontent"
)

const (
	managedLabelKey  = "run.package-operator.managed-objects"
	externalLabelKey = "run.package-operator.external-objects"
)

func Image(pkg *packagecontent.Package) (v1.Image, error) {
	managedBytes, err := json.Marshal(pkg.Metadata.ManagedObjectTypes)
	if err != nil {
		panic(err)
	}
	externalBytes, err := json.Marshal(pkg.Metadata.ExternalObjectTypes)
	if err != nil {
		panic(err)
	}
	annotations := map[string]string{managedLabelKey: string(managedBytes), externalLabelKey: string(externalBytes)}

	image := mutate.Annotations(empty.Image, annotations).(v1.Image)

	subFiles := packagecontent.Files{}
	for k, v := range pkg.Files {
		subFiles[packages.AddPathPrefix(k)] = v
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

func PushedImage(ctx context.Context, references []string, pkg *packagecontent.Package, opts ...crane.Option) error {
	image, err := Image(pkg)
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
