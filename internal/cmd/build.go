package cmd

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/crane"

	"package-operator.run/internal/packages"
)

type BuildValidationError struct {
	Msg string
}

func (u BuildValidationError) Error() string {
	return u.Msg
}

func NewBuild(opts ...BuildOption) *Build {
	var cfg BuildConfig

	cfg.Option(opts...)
	cfg.Default()

	return &Build{
		cfg: cfg,
	}
}

type Build struct {
	cfg BuildConfig
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

	pkg, err := packages.DefaultStructuralLoader.Load(ctx, rawPkg)
	if err != nil {
		return fmt.Errorf("loading package from files: %w", err)
	}

	var craneOpts []crane.Option
	if cfg.Insecure {
		craneOpts = append(craneOpts, crane.Insecure)
	}

	validators := append(
		packages.PackageValidatorList{
			&packages.LockfileDigestLookupValidator{
				CraneOptions: craneOpts,
			},
			packages.NewTemplateTestValidator(srcPath),
		},
		packages.DefaultPackageValidators...,
	)
	if err := validators.ValidatePackage(ctx, pkg); err != nil {
		return fmt.Errorf("loading package from files: %w", err)
	}

	if cfg.OutputPath != "" {
		b.cfg.Log.Info("writing tagged image to disk", "path", cfg.OutputPath)

		if err := packages.ToOCIFile(cfg.OutputPath, cfg.Tags, rawPkg); err != nil {
			return fmt.Errorf("exporting package to file: %w", err)
		}
	}

	if cfg.Push {
		if err := packages.ToPushedOCI(ctx, cfg.Tags, rawPkg, craneOpts...); err != nil {
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
