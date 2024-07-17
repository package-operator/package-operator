package packageimport

import (
	"archive/tar"
	"errors"
	"io"
	"path/filepath"
	"strings"

	containerregistrypkgv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"package-operator.run/internal/packages/internal/packagetypes"
)

const (
	olmManifestFolder = "manifests"
	olmMetadataFolder = "metadata"
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
