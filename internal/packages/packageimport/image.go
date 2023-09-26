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

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/packages"
	"package-operator.run/internal/packages/packagecontent"
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

		path, err := packages.StripPathPrefix(hdr.Name)
		if err != nil {
			return nil, err
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

func HelmImage(ctx context.Context, image v1.Image) (m packagecontent.Files, err error) {
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

		// Helm OCI images store the chart in a subfolder with the same name as the chart.
		// Because we don't know the chart name here, we are just stripping the first folder level.
		segments := strings.Split(hdr.Name, string(filepath.Separator))
		if len(segments) < 2 {
			continue
		}
		path := filepath.Join(segments[1:]...)

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

func PulledImage(ctx context.Context, ref string, pkgType corev1alpha1.PackageType) (packagecontent.Files, error) {
	puller := NewPuller()

	return puller.Pull(ctx, ref, pkgType)
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

func (p *Puller) Pull(ctx context.Context, ref string, pkgType corev1alpha1.PackageType, opts ...PullOption) (packagecontent.Files, error) {
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

	if pkgType == corev1alpha1.PackageTypeHelm {
		return HelmImage(ctx, img)
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
