package packageimport

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	containerregistrypkgv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"package-operator.run/internal/packages/internal/packagetypes"
)

// Imports a RawPackage from the given OCI image.
func FromOCI(ctx context.Context, image containerregistrypkgv1.Image) (
	rawPkg *packagetypes.RawPackage, err error,
) {
	files := packagetypes.Files{}
	reader := mutate.Extract(image)
	verboseLog := logr.FromContextOrDiscard(ctx).V(1)

	defer func() {
		if cErr := reader.Close(); err == nil && cErr != nil {
			err = cErr
		}
	}()
	tarReader := tar.NewReader(reader)

	for {
		hdr, err := tarReader.Next()
		if err != nil && errors.Is(err, io.EOF) {
			break
		}

		path, err := stripOCIPathPrefix(hdr.Name)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(path, "../") {
			continue
		}

		if isFilePathToBeExcluded(path) {
			verboseLog.Info("skipping file in source", "path", path)
			continue
		}

		data, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, fmt.Errorf("read file header from layer: %w", err)
		}

		files[path] = data
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

func stripOCIPathPrefix(path string) (string, error) {
	strippedPath, err := filepath.Rel(packagetypes.OCIPathPrefix, path)
	if err != nil {
		return strippedPath, fmt.Errorf(
			"package image contains files not under the dir %s: %w", packagetypes.OCIPathPrefix, err)
	}

	return strippedPath, nil
}

func isFilePathToBeExcluded(path string) bool {
	for _, pathSegment := range strings.Split(
		path, string(filepath.Separator)) {
		if strings.HasPrefix(pathSegment, ".") {
			return true
		}
	}
	return false
}
