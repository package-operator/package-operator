package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"k8s.io/apimachinery/pkg/runtime"

	"package-operator.run/internal/packages"
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
	Log  logr.Logger
	Pull PullFn
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
	if c.Pull == nil {
		c.Pull = packages.FromRegistry
	}
}

type ValidateOption interface {
	ConfigureValidate(*ValidateConfig)
}

type PullFn func(ctx context.Context, ref string, opts ...crane.Option) (*packages.RawPackage, error)

func (v *Validate) ValidatePackage(ctx context.Context, opts ...ValidatePackageOption) error {
	var cfg ValidatePackageConfig

	cfg.Option(opts...)
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validating options: %w", err)
	}

	var (
		rawPkg     *packages.RawPackage
		validators = packages.DefaultPackageValidators
	)

	if cfg.Path != "" {
		var err error

		rawPkg, err = getPackageFromPath(ctx, cfg.Path)
		if err != nil {
			return fmt.Errorf("getting package from path: %w", err)
		}

		validators = append(validators, packages.NewTemplateTestValidator(cfg.Path))
	} else {
		var err error

		rawPkg, err = v.getPackageFromRemoteRef(ctx, cfg)
		if err != nil {
			return fmt.Errorf("getting package from remote reference: %w", err)
		}
	}

	pkg, err := packages.DefaultStructuralLoader.Load(ctx, rawPkg)
	if err != nil {
		return err
	}

	if err := validators.ValidatePackage(ctx, pkg); err != nil {
		return fmt.Errorf("loading package from files: %w", err)
	}

	return nil
}

func getPackageFromPath(ctx context.Context, path string) (*packages.RawPackage, error) {
	rawPkg, err := packages.FromFolder(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("importing package from folder: %w", err)
	}
	return rawPkg, nil
}

func (v *Validate) getPackageFromRemoteRef(
	ctx context.Context, cfg ValidatePackageConfig,
) (*packages.RawPackage, error) {
	ref, err := name.ParseReference(cfg.RemoteReference)
	if err != nil {
		return nil, fmt.Errorf("parsing remote reference: %w", err)
	}

	var opts []crane.Option
	if cfg.Insecure {
		opts = append(opts, crane.Insecure)
	}

	rawPkg, err := v.cfg.Pull(ctx, ref.String(), opts...)
	if err != nil {
		return nil, fmt.Errorf("importing package from image: %w", err)
	}

	return rawPkg, nil
}

type ValidatePackageConfig struct {
	Insecure        bool
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
