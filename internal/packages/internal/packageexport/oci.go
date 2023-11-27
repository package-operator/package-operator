package packageexport

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/crane"
	containerregistrypkgv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"package-operator.run/internal/packages/internal/packagetypes"
)

// Exports the package as OCI (Open Container Image).
func ToOCI(pkg *packagetypes.RawPackage) (containerregistrypkgv1.Image, error) {
	// Hardcoded to linux/amd64 or kubernetes will refuse to pull the image on
	// our target architecture. We will drop this after refactoring our in-cluster
	// package loading process to make it architecture agnostic.
	configFile := &containerregistrypkgv1.ConfigFile{
		Architecture: "amd64",
		OS:           "linux",
		Config:       containerregistrypkgv1.Config{},
		RootFS:       containerregistrypkgv1.RootFS{Type: "layers"},
	}
	image, err := mutate.ConfigFile(empty.Image, configFile)
	if err != nil {
		return nil, err
	}

	subFiles := map[string][]byte{}
	for k, v := range pkg.Files {
		subFiles[addOCIPathPrefix(k)] = v
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

	var managed []string
	for m := range pkg.Permissions.Managed {
		managed = append(managed, pkg.Permissions.Managed[m].String())
	}
	var external []string
	for e := range pkg.Permissions.External {
		external = append(external, pkg.Permissions.External[e].String())
	}

	annotations := map[string]string{
		"managed":  strings.Join(managed, ","),
		"external": strings.Join(external, ","),
	}

	image = mutate.MediaType(image, types.OCIManifestSchema1)
	image = mutate.ConfigMediaType(image, types.OCIConfigJSON)
	image = mutate.Annotations(image, annotations).(containerregistrypkgv1.Image)

	return image, nil
}

// Exports the given package to an OCI tar under the given name and tags.
func ToOCIFile(dst string, tags []string, pkg *packagetypes.RawPackage) error {
	image, err := ToOCI(pkg)
	if err != nil {
		return err
	}

	m := map[string]containerregistrypkgv1.Image{}
	for _, tag := range tags {
		m[tag] = image
	}
	if err := crane.MultiSave(m, dst); err != nil {
		return fmt.Errorf("dump to %s: %w", dst, err)
	}

	return nil
}

// Exports the given package by pushing it to an OCI registry.
func ToPushedOCI(ctx context.Context, references []string, pkg *packagetypes.RawPackage, opts ...crane.Option) error {
	image, err := ToOCI(pkg)
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

func addOCIPathPrefix(path string) string {
	return filepath.Join(packagetypes.OCIPathPrefix, path)
}
