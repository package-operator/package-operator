package packagebytes

import (
	"fmt"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
)

type Saver struct{}

func NewSaver() *Saver {
	return &Saver{}
}

func (s Saver) ToImage(fileMap FileMap) (v1.Image, error) {
	configFile := &v1.ConfigFile{Architecture: "amd64", OS: "linux", Config: v1.Config{}, RootFS: v1.RootFS{Type: "layers"}}
	image, err := mutate.ConfigFile(empty.Image, configFile)
	if err != nil {
		return nil, err
	}

	subFileMap := FileMap{}
	for k, v := range fileMap {
		subFileMap[filepath.Join(tarPrefixPath, k)] = v
	}

	layer, err := crane.Layer(subFileMap)
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

	return image, nil
}
