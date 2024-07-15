package packageimport

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"testing/fstest"

	containerregistrypkgv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"package-operator.run/internal/packages/internal/packagekickstart"
	"package-operator.run/internal/packages/internal/packagekickstart/rukpak/convert"
	"package-operator.run/internal/packages/internal/packagetypes"
)

const (
	olmManifestFolder     = "manifests"
	olmMetadataFolder     = "metadata"
	convertedManifestFile = "manifests/manifest.yaml"
)

// peeks image contents to see if it is an OLM bundle image.
func peekIsOLM(image containerregistrypkgv1.Image) (isOLM bool, err error) {
	var (
		packageManifestFound bool
		manifestsFolderFound bool
		metadataFolderFound  bool
	)

	reader := mutate.Extract(image)
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

		pkgManifestPath := filepath.Join(packagetypes.OCIPathPrefix, packagetypes.PackageManifestFilename)
		switch hdr.Name {
		case pkgManifestPath + ".yml", pkgManifestPath + ".yaml":
			packageManifestFound = true
		}
		if strings.HasPrefix(hdr.Name, olmManifestFolder+"/") {
			manifestsFolderFound = true
		}
		if strings.HasPrefix(hdr.Name, olmMetadataFolder+"/") {
			metadataFolderFound = true
		}
	}
	return !packageManifestFound && manifestsFolderFound && metadataFolderFound, nil
}

func FromOLMBundleImage(ctx context.Context, image containerregistrypkgv1.Image) (
	rawPkg *packagetypes.RawPackage, err error,
) {
	rawFS := fstest.MapFS{}
	reader := mutate.Extract(image)

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

		path := hdr.Name
		if strings.HasPrefix(path, "../") {
			continue
		}

		data, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, fmt.Errorf("read file header from layer: %w", err)
		}

		rawFS[path] = &fstest.MapFile{
			Data: data,
		}
	}

	if len(rawFS) == 0 {
		return nil, packagetypes.ErrEmptyPackage
	}

	convertedFS, reg, err := convert.RegistryV1ToPlain(rawFS, "", nil)
	if err != nil {
		return nil, fmt.Errorf("converting OLM Bundle to static manifests: %w", err)
	}
	manifestBytes, err := fs.ReadFile(convertedFS, convertedManifestFile)
	if err != nil {
		return nil, fmt.Errorf("reading converted manifests: %w", err)
	}

	rawPkg, _, err = packagekickstart.KickstartFromBytes(ctx, reg.PackageName, manifestBytes, packagekickstart.KickstartOptions{})
	return
}
