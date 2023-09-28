package packageimport

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

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

func PulledImage(ctx context.Context, ref string, opts ...PullOption) (packagecontent.Files, error) {
	puller := NewPuller()

	return puller.Pull(ctx, ref, opts...)
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
	return &Puller{
		cranePull: crane.Pull,
		image:     Image,
	}
}

type cranePullFn func(src string, opt ...crane.Option) (v1.Image, error)

type imageFn func(ctx context.Context, image v1.Image) (m packagecontent.Files, err error)

type Puller struct {
	cranePull cranePullFn
	image     imageFn
}

type dockerConfigJSON struct {
	Auths map[string]authn.AuthConfig
}

func (p *Puller) Pull(ctx context.Context, ref string, opts ...PullOption) (packagecontent.Files, error) {
	var cfg pullConfig

	cfg.Option(opts...)

	craneOpts := []crane.Option{}

	// Prepare authenticator(s) if pull secret was specified.
	if len(cfg.PullSecret) != 0 {
		var dockerConfig dockerConfigJSON

		if err := json.Unmarshal(cfg.PullSecret, &dockerConfig); err != nil {
			return nil, err
		}

		for _, auth := range dockerConfig.Auths {
			craneOpts = append(craneOpts, crane.WithAuth(authn.FromConfig(auth)))
		}
	}

	if cfg.Insecure {
		craneOpts = append(craneOpts, crane.Insecure)
	}

	img, err := p.cranePull(ref, craneOpts...)
	if err != nil {
		return nil, err
	}

	return p.image(ctx, img)
}
