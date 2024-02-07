package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"k8s.io/apimachinery/pkg/runtime"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages"
	"package-operator.run/internal/utils"
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

	pkg, err := packages.DefaultStructuralLoader.Load(ctx, rawPkg)
	if err != nil {
		return fmt.Errorf("loading package from files: %w", err)
	}
	if err := b.ValidatePackage(ctx, pkg); err != nil {
		return fmt.Errorf("loading package from files: %w", err)
	}

	validators := append(
		packages.PackageValidatorList{
			packages.NewTemplateTestValidator(filepath.Join(srcPath, ".test-fixtures")),
		},
		packages.DefaultPackageValidators...,
	)
	if err := validators.ValidatePackage(ctx, pkg); err != nil {
		return fmt.Errorf("loading package from files: %w", err)
	}

	rawPkg.Permissions, err = packages.Permissions(ctx, rawPkg.Files)

	if cfg.OutputPath != "" {
		b.cfg.Log.Info("writing tagged image to disk", "path", cfg.OutputPath)

		if err := packages.ToOCIFile(cfg.OutputPath, cfg.Tags, rawPkg); err != nil {
			return fmt.Errorf("exporting package to file: %w", err)
		}
	}

	if cfg.Push {
		var craneOpts []crane.Option

		if cfg.Insecure {
			craneOpts = append(craneOpts, crane.Insecure)
		}

		if err := packages.ToPushedOCI(ctx, cfg.Tags, rawPkg, craneOpts...); err != nil {
			return fmt.Errorf("exporting package to image: %w", err)
		}
	}

	return nil
}

func (b *Build) ValidatePackage(_ context.Context, pkg *packages.Package) error {
	if pkg.ManifestLock == nil {
		if len(pkg.Manifest.Spec.Images) > 0 {
			return err(`manifest.lock.yaml is missing (try running "kubectl package update")`)
		}
		return nil
	}

	pkgImages := map[string]manifests.PackageManifestImage{}
	for _, image := range pkg.Manifest.Spec.Images {
		pkgImages[image.Name] = image
	}
	pkgLockImages := map[string]manifests.PackageManifestLockImage{}
	for _, image := range pkg.ManifestLock.Spec.Images {
		pkgLockImages[image.Name] = image
	}

	// check that all the images in manifest file exists in lock files too, and their "image" fields are the same
	for imageName, image := range pkgImages {
		lockImage, existsInLock := pkgLockImages[imageName]
		if !existsInLock {
			return err(`image %q exists in manifest but not in lock file (try running "kubectl package update")`, imageName)
		}
		if image.Image != lockImage.Image {
			return err(
				`tags for image %q differ between manifest and lock file: %q vs %q (try running "kubectl package update")`,
				imageName, image.Image, lockImage.Image)
		}
	}

	// check that all the images in lock file exists in manifest files too (which ensures manifest images == lock images)
	for imageName := range pkgLockImages {
		_, existsInManifest := pkgImages[imageName]
		if !existsInManifest {
			return err(`image %q exists in lock but not in manifest file (try running "kubectl package update")`, imageName)
		}
	}

	// validate digests
	for imageName, lockImage := range pkgLockImages {
		overriddenImage, err := utils.ImageURLWithOverrideFromEnv(lockImage.Image)
		if err != nil {
			return fmt.Errorf("%w: can't parse image %q reference %q", err, imageName, lockImage.Image)
		}
		ref, err := name.ParseReference(overriddenImage)
		if err != nil {
			return fmt.Errorf("%w: can't parse image %q reference %q", err, imageName, lockImage.Image)
		}
		digestRef := ref.Context().Digest(lockImage.Digest)
		if _, err := b.cfg.Resolver.ResolveDigest(digestRef.String()); err != nil {
			return fmt.Errorf("%w: image %q digest error (%q)", err, imageName, lockImage.Digest)
		}
	}

	return nil
}

func err(format string, a ...any) *BuildValidationError {
	return &BuildValidationError{
		Msg: fmt.Sprintf(format, a...),
	}
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
