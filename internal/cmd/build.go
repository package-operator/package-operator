package cmd

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/crane"
	"k8s.io/apimachinery/pkg/runtime"

	"package-operator.run/internal/packages"
	"package-operator.run/pkg/packaging"
)

type BuildValidationError struct {
	Msg string
}

func (u BuildValidationError) Error() string {
	return u.Msg
}

func NewBuild(scheme *runtime.Scheme, opts ...BuildOption) *Build {
	var cfg BuildConfig

	cfg.Option(opts...)
	cfg.Default()

	return &Build{
		cfg:    cfg,
		scheme: scheme,
	}
}

type Build struct {
	cfg    BuildConfig
	scheme *runtime.Scheme
}

type BuildConfig struct {
	Log      logr.Logger
	Resolver DigestResolver
}

func (c *BuildConfig) Option(opts ...BuildOption) {
	for _, opt := range opts {
		opt.ConfigureBuild(c)
	}
}

func (c *BuildConfig) Default() {
	if c.Log.GetSink() == nil {
		c.Log = logr.Discard()
	}

	if c.Resolver == nil {
		c.Resolver = &defaultDigestResolver{}
	}
}

type BuildOption interface {
	ConfigureBuild(*BuildConfig)
}

func (b *Build) BuildFromSource(ctx context.Context, srcPath string, opts ...BuildFromSourceOption) error {
	b.cfg.Log.Info("loading source from disk", "path", srcPath)

	var cfg BuildFromSourceConfig

	cfg.Option(opts...)

	rawPkg, err := getPackageFromPath(ctx, srcPath)
	if err != nil {
		return fmt.Errorf("load source from disk path %s: %w", srcPath, err)
	}

	b.cfg.Log.Info("creating image")

	pkg, err := packaging.Load(ctx, rawPkg)
	if err != nil {
		return fmt.Errorf("loading package from files: %w", err)
	}

	var (
		registryOpts []packaging.RegistryOption
		craneOpts    []crane.Option
	)
	if cfg.Insecure {
		registryOpts = append(registryOpts, packaging.WithInsecure{})
		craneOpts = append(craneOpts, crane.Insecure)
	}

	if err := packaging.Validate(ctx, pkg, packaging.WithPackageValidators{
		&packages.LockfileDigestLookupValidator{
			CraneOptions: craneOpts,
		},
	}); err != nil {
		return err
	}

	if cfg.OutputPath != "" {
		b.cfg.Log.Info("writing tagged image to disk", "path", cfg.OutputPath)

		if err := packaging.ToOCIFile(cfg.OutputPath, cfg.Tags, rawPkg); err != nil {
			return fmt.Errorf("exporting package to file: %w", err)
		}
	}

	if cfg.Push {
		if err := packaging.ToPushedOCI(ctx, cfg.Tags, rawPkg, registryOpts...); err != nil {
			return fmt.Errorf("exporting package to image: %w", err)
		}
	}

	return nil
}

type BuildFromSourceConfig struct {
	Insecure   bool
	OutputPath string
	Tags       []string
	Push       bool
}

func (c *BuildFromSourceConfig) Option(opts ...BuildFromSourceOption) {
	for _, opt := range opts {
		opt.ConfigureBuildFromSource(c)
	}
}

type BuildFromSourceOption interface {
	ConfigureBuildFromSource(*BuildFromSourceConfig)
}
