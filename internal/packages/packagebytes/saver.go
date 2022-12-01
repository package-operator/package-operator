package packagebytes

import (
	"bytes"
	"fmt"
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

type Saver struct{}

func NewSaver() *Saver {
	return &Saver{}
}

func (s Saver) ToImage(fileMap FileMap) (v1.Image, error) {
	tarData, err := toTar(fileMap)
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
