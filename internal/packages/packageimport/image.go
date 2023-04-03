package packageimport

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/packages/packagecontent"
)

func Image(_ context.Context, image v1.Image) (m packagecontent.Files, err error) {
	files := packagecontent.Files{}
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

		tarPath := hdr.Name
		path, err := filepath.Rel(packages.ImageFilePrefixPath, tarPath)
		if err != nil {
			return nil, fmt.Errorf("package image contains files not under the dir %s: %w", packages.ImageFilePrefixPath, err)
		}

		data, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, fmt.Errorf("read file header from layer: %w", err)
		}

		files[path] = data
	}

	return files, nil
}

func PulledImage(ctx context.Context, ref string) (packagecontent.Files, error) {
	img, err := crane.Pull(ref)
	if err != nil {
		return nil, err
	}

	return Image(ctx, img)
}
