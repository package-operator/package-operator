package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"k8s.io/apimachinery/pkg/runtime"

	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/packages/packageimport"
	"package-operator.run/package-operator/internal/packages/packageloader"
)

func NewValidate(scheme *runtime.Scheme, opts ...ValidateOption) *Validate {
	var cfg ValidateConfig

	cfg.Option(opts...)
	cfg.Default()

	return &Validate{
		cfg:    cfg,
		scheme: scheme,
	}
}

type Validate struct {
	cfg    ValidateConfig
	scheme *runtime.Scheme
}

type ValidateConfig struct {
	Log    logr.Logger
	Puller PackagePuller
}

func (c *ValidateConfig) Option(opts ...ValidateOption) {
	for _, opt := range opts {
		opt.ConfigureValidate(c)
	}
}

func (c *ValidateConfig) Default() {
	if c.Log.GetSink() == nil {
		c.Log = logr.Discard()
	}
	if c.Puller == nil {
		c.Puller = &defaultPackagePuller{}
	}
}

type ValidateOption interface {
	ConfigureValidate(*ValidateConfig)
}

type PackagePuller interface {
	PullPackage(ctx context.Context, ref string) (packagecontent.Files, error)
}

func (v *Validate) ValidatePackage(ctx context.Context, opts ...ValidatePackageOption) error {
	var cfg ValidatePackageConfig

	cfg.Option(opts...)
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validating options: %w", err)
	}

	var (
		filemap   packagecontent.Files
		extraOpts []packageloader.Option
	)

	if cfg.Path != "" {
		var err error

		filemap, extraOpts, err = getPackageFromPath(ctx, v.scheme, cfg.Path)
		if err != nil {
			return fmt.Errorf("getting package from path: %w", err)
		}
	} else {
		var err error

		filemap, err = v.getPackageFromRemoteRef(ctx, cfg.RemoteReference)
		if err != nil {
			return fmt.Errorf("getting package from remote reference: %w", err)
		}
	}

	if _, err := packageloader.New(v.scheme, extraOpts...).FromFiles(ctx, filemap); err != nil {
		return fmt.Errorf("loading package from files: %w", err)
	}

	return nil
}

func getPackageFromPath(ctx context.Context, scheme *runtime.Scheme, path string) (packagecontent.Files, []packageloader.Option, error) {
	filemap, err := packageimport.Folder(ctx, path)
	if err != nil {
		return nil, nil, fmt.Errorf("importing package from folder: %w", err)
	}

	ttv := packageloader.NewTemplateTestValidator(scheme, filepath.Join(path, ".test-fixtures"))

	return filemap, []packageloader.Option{packageloader.WithPackageAndFilesValidators(ttv)}, nil
}

func (v *Validate) getPackageFromRemoteRef(ctx context.Context, remoteRef string) (packagecontent.Files, error) {
	ref, err := name.ParseReference(remoteRef)
	if err != nil {
		return nil, fmt.Errorf("parsing remote reference: %w", err)
	}

	filemap, err := v.cfg.Puller.PullPackage(ctx, ref.String())
	if err != nil {
		return nil, fmt.Errorf("importing package from image: %w", err)
	}

	return filemap, nil
}

type ValidatePackageConfig struct {
	Path            string
	RemoteReference string
}

func (c *ValidatePackageConfig) Option(opts ...ValidatePackageOption) {
	for _, opt := range opts {
		opt.ConfigureValidatePackage(c)
	}
}

var ErrInvalidOptions = errors.New("invalid options")

func (c *ValidatePackageConfig) Validate() error {
	if c.Path == "" && c.RemoteReference == "" {
		return fmt.Errorf("%w: either 'Path' or 'RemoteReference' must be provided", ErrInvalidOptions)
	}
	if c.Path != "" && c.RemoteReference != "" {
		return fmt.Errorf("%w: 'Path' and 'RemoteReference' are mutually exclusive", ErrInvalidOptions)
	}

	return nil
}

type ValidatePackageOption interface {
	ConfigureValidatePackage(*ValidatePackageConfig)
}

type defaultPackagePuller struct{}

func (p *defaultPackagePuller) PullPackage(ctx context.Context, ref string) (packagecontent.Files, error) {
	return packageimport.PulledImage(ctx, ref)
}
