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
	"package-operator.run/internal/packages/internal/packagekickstart/rukpak/convert"
	"package-operator.run/internal/packages/internal/packagetypes"
	"pkg.package-operator.run/cardboard/kubeutils/kubemanifests"
)

const convertedManifestFile = "manifests/manifest.yaml"

func ImportOLMBundleImage(ctx context.Context, image containerregistrypkgv1.Image) (
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

	fmt.Println("HAAAALO:", rawFS)
	fmt.Printf("HAAAALOOOO: %#v\n", rawFS["manifests"])
	d, err := fs.ReadDir(rawFS, "manifests")
	fmt.Println("HAAAALO2: ", d, err)
	ffff := bigFS(&rawFS)
	d, err = fs.ReadDir(ffff, "manifests")
	fmt.Println("HAAAALO3: ", d, err)

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

type bigFS interface {
	fs.FS
	fs.ReadDirFS
	fs.ReadFileFS
}

func FromOLMBundleImage(ctx context.Context, image containerregistrypkgv1.Image) (
	rawPkg *packagetypes.RawPackage, err error,
) {
	objs, reg, err := ImportOLMBundleImage(ctx, image)
	if err != nil {
		return nil, err
	}

	rawPkg, _, err = Kickstart(ctx, reg.PackageName, objs, KickstartOptions{
		Parametrize: []string{"env", "replicas", "tolerations", "nodeSelectors", "resources"},
	})
	return
}
