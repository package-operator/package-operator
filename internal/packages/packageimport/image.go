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
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/packages/packagecontent"
)

func Image(ctx context.Context, image v1.Image) (m packagecontent.Files, err error) {
	files := packagecontent.Files{}
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

		tarPath := hdr.Name
		path, err := filepath.Rel(packages.ImageFilePrefixPath, tarPath)
		if err != nil {
			return nil, fmt.Errorf("package image contains files not under the dir %s: %w", packages.ImageFilePrefixPath, err)
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

	return files, nil
}

func PulledImage(ctx context.Context, ref string) (packagecontent.Files, error) {
	puller := NewPuller()

	return puller.Pull(ctx, ref)
}

func isFilePathToBeExcluded(path string) bool {
	for _, pathSegment := range strings.Split(
		path, string(filepath.Separator)) {
		if isFilenameToBeExcluded(pathSegment) {
			return true
		}
	}
	return false
}

func NewPuller() *Puller {
	return &Puller{}
}

type Puller struct{}

func (p *Puller) Pull(ctx context.Context, ref string, opts ...PullOption) (packagecontent.Files, error) {
	var cfg PullConfig

	cfg.Option(opts...)

	var craneOpts []crane.Option
	if cfg.Insecure {
		craneOpts = append(craneOpts, crane.Insecure)
	}

	img, err := crane.Pull(ref, craneOpts...)
	if err != nil {
		return nil, err
	}

	return Image(ctx, img)
}

type PullConfig struct {
	Insecure bool
}

func (c *PullConfig) Option(opts ...PullOption) {
	for _, opt := range opts {
		opt.ConfigurePull(c)
	}
}

type PullOption interface {
	ConfigurePull(*PullConfig)
}
