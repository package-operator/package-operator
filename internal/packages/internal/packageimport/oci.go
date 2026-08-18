package packageimport

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	containerregistrypkgv1 "github.com/google/go-containerregistry/pkg/v1"

	"package-operator.run/internal/packages/internal/packagetypes"
)

// Imports a RawPackage from the given OCI image.
func FromOCI(ctx context.Context, image containerregistrypkgv1.Image) (
	rawPkg *packagetypes.RawPackage, err error,
) {
	files := packagetypes.Files{}
	verboseLog := logr.FromContextOrDiscard(ctx).V(1)

	layers, err := image.Layers()
	if err != nil {
		return nil, fmt.Errorf("read image layers: %w", err)
	}

	// Read layers in order so later layers override earlier ones. Do not use
	// mutate.Extract: since go-containerregistry v0.21.8 a "." tar entry is
	// treated as a root whiteout and hides every package file.
	for _, layer := range layers {
		if err := appendFilesFromLayer(verboseLog, layer, files); err != nil {
			return nil, err
		}
	}

	if len(files) == 0 {
		return nil, packagetypes.ErrEmptyPackage
	}

	cf, err := image.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("get configFile for Image: %w", err)
	}

	return &packagetypes.RawPackage{
		Files:  files,
		Labels: cf.Config.Labels,
	}, nil
}

func appendFilesFromLayer(
	verboseLog logr.Logger, layer containerregistrypkgv1.Layer, files packagetypes.Files,
) (err error) {
	reader, err := layer.Uncompressed()
	if err != nil {
		return fmt.Errorf("read layer contents: %w", err)
	}
	defer func() {
		if cErr := reader.Close(); err == nil && cErr != nil {
			err = cErr
		}
	}()

	tarReader := tar.NewReader(reader)
	for {
		hdr, nextErr := tarReader.Next()
		if nextErr != nil {
			if errors.Is(nextErr, io.EOF) {
				break
			}
			return fmt.Errorf("read file header from layer: %w", nextErr)
		}

		if hdr.Typeflag == tar.TypeDir {
			continue
		}

		name := path.Clean(strings.ReplaceAll(hdr.Name, "\\", "/"))
		if name == "." || name == ".." || strings.HasPrefix(name, "../") {
			continue
		}

		pkgPath, err := stripOCIPathPrefix(hdr.Name)
		if err != nil {
			return err
		}
		if strings.HasPrefix(pkgPath, "../") {
			continue
		}

		if isFilePathToBeExcluded(pkgPath) {
			verboseLog.Info("skipping file in source", "path", pkgPath)
			continue
		}

		data, err := io.ReadAll(tarReader)
		if err != nil {
			return fmt.Errorf("read file contents from layer: %w", err)
		}

		files[pkgPath] = data
	}

	return nil
}

func stripOCIPathPrefix(path string) (string, error) {
	strippedPath, err := filepath.Rel(packagetypes.OCIPathPrefix, path)
	if err != nil {
		return strippedPath, fmt.Errorf(
			"package image contains files not under the dir %s: %w", packagetypes.OCIPathPrefix, err)
	}

	return strippedPath, nil
}

func isFilePathToBeExcluded(path string) bool {
	for pathSegment := range strings.SplitSeq(
		path, string(filepath.Separator)) {
		if strings.HasPrefix(pathSegment, ".") {
			return true
		}
	}
	return false
}
