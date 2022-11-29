package filemap

import (
	"bytes"
	"fmt"
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func ToImage(fileMap FileMap) (v1.Image, error) {
	tarData, err := ToTar(fileMap)
	if err != nil {
		return nil, err
	}

	opener := func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(tarData)), nil }

	layer, err := tarball.LayerFromOpener(opener, tarball.WithCompressedCaching)
	if err != nil {
		return nil, fmt.Errorf("create layer from tar: %w", err)
	}

	image, err := mutate.Append(empty.Image, mutate.Addendum{Layer: layer})
	if err != nil {
		return nil, fmt.Errorf("create image from layer: %w", err)
	}

	return image, nil
}

func FromImage(image v1.Image) (FileMap, error) {
	layers, err := image.Layers()
	if err != nil {
		return nil, fmt.Errorf("access layers: %w", err)
	}

	fileMap := FileMap{}

	for _, layer := range layers {
		reader, err := layer.Uncompressed()
		if err != nil {
			return nil, fmt.Errorf("access layer: %w", err)
		}

		layerFileMap, err := FromTaredReader(reader)
		if err != nil {
			return nil, err
		}

		for k, v := range layerFileMap {
			fileMap[k] = v
		}
	}

	return fileMap, nil
}
