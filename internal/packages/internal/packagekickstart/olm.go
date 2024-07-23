package packagekickstart

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"testing/fstest"

	containerregistrypkgv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"pkg.package-operator.run/cardboard/kubeutils/kubemanifests"

	"package-operator.run/internal/packages/internal/packagekickstart/rukpak/convert"
	"package-operator.run/internal/packages/internal/packagetypes"
)

const convertedManifestFile = "manifests/manifest.yaml"

// ImportOLMBundleImage takes an OLM registry v1 bundle OCI,
// converts it into static manifests and returns a list all objects contained.
func ImportOLMBundleImage(_ context.Context, image containerregistrypkgv1.Image) (
	objects []unstructured.Unstructured, reg convert.RegistryV1, err error,
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
		if hdr.Typeflag == tar.TypeDir {
			continue
		}

		data, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, reg, fmt.Errorf("read file header from layer: %w", err)
		}

		rawFS[path] = &fstest.MapFile{
			Data: data,
		}
	}

	if len(rawFS) == 0 {
		return nil, reg, packagetypes.ErrEmptyPackage
	}

	convertedFS, reg, err := convert.RegistryV1ToPlain(rawFS, "", nil)
	if err != nil {
		return nil, reg, fmt.Errorf("converting OLM Bundle to static manifests: %w", err)
	}
	manifestBytes, err := fs.ReadFile(convertedFS, convertedManifestFile)
	if err != nil {
		return nil, reg, fmt.Errorf("reading converted manifests: %w", err)
	}
	objects, err = kubemanifests.LoadKubernetesObjectsFromBytes(manifestBytes)
	if err != nil {
		return nil, reg, fmt.Errorf("loading objects from manifests: %w", err)
	}
	return objects, reg, nil
}
