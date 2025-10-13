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
	var cfg BuildFromSourceConfig

	cfg.Default()
	cfg.Option(opts...)

	var log logr.Logger
	switch cfg.OutputFormat {
	case OutputFormatHuman:
		log = b.cfg.Log
	case OutputFormatDigest:
		log = logr.Discard()
	default:
		panic("unknown output format: " + cfg.OutputFormat)
	}

	log.Info("loading source from disk", "path", srcPath)

	rawPkg, err := getPackageFromPath(ctx, srcPath)
	if err != nil {
		return fmt.Errorf("load source from disk path %s: %w", srcPath, err)
	}

	log.Info("creating image")

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
		log.Info("writing tagged image to disk", "path", cfg.OutputPath)

		if err := packages.ToOCIFile(cfg.OutputPath, cfg.Tags, rawPkg); err != nil {
			return fmt.Errorf("exporting package to file: %w", err)
		}
	}

	if cfg.Push {
		digest, err := packages.ToPushedOCI(ctx, cfg.Tags, rawPkg, craneOpts...)
		if err != nil {
			return fmt.Errorf("exporting package to image: %w", err)
		}

		if cfg.OutputFormat == OutputFormatDigest {
			fmt.Println(digest) //nolint:forbidigo
		}
	}

	return nil
}

type BuildFromSourceConfig struct {
	Insecure     bool
	OutputPath   string
	OutputFormat string
	Tags         []string
	Push         bool
}

func (c *BuildFromSourceConfig) Option(opts ...BuildFromSourceOption) {
	for _, opt := range opts {
		opt.ConfigureBuildFromSource(c)
	}
}

func (c *BuildFromSourceConfig) Default() {
	if c.OutputFormat == "" {
		c.OutputFormat = OutputFormatHuman
	}
}

type BuildFromSourceOption interface {
	ConfigureBuildFromSource(*BuildFromSourceConfig)
}
