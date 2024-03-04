package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages"
	"package-operator.run/internal/utils"
)

func NewUpdate(opts ...UpdateOption) *Update {
	var cfg UpdateConfig

	cfg.Option(opts...)

	return &Update{cfg: cfg}
}

type Update struct {
	cfg UpdateConfig
}

type UpdateConfig struct {
	Log      logr.Logger
	Clock    Clock
	Loader   PackageLoader
	Resolver DigestResolver
}

func (c *UpdateConfig) Option(opts ...UpdateOption) {
	for _, opt := range opts {
		opt.ConfigureUpdate(c)
	}
}

func clockOrDefaultClock(c Clock) Clock {
	if c == nil {
		c = &defaultClock{}
	}
	return c
}

func loaderOrDefaultLoader(p PackageLoader) PackageLoader {
	if p == nil {
		p = NewDefaultPackageLoader(runtime.NewScheme())
	}

	return p
}

func resolverOrDefaultResolver(r DigestResolver) DigestResolver {
	if r == nil {
		r = &defaultDigestResolver{}
	}

	return r
}

type UpdateOption interface {
	ConfigureUpdate(*UpdateConfig)
}

func (u Update) UpdateLockData(ctx context.Context, srcPath string, opts ...GenerateLockDataOption) error {
	data, err := u.GenerateLockData(ctx, srcPath, opts...)
	if err != nil {
		return err
	}
	lockFilePath := filepath.Join(srcPath, packages.PackageManifestLockFilename+".yaml")
	if err := os.WriteFile(lockFilePath, data, 0o644); err != nil {
		return fmt.Errorf("writing lock file: %w", err)
	}

	return nil
}

func (u Update) GenerateLockData(ctx context.Context, srcPath string, opts ...GenerateLockDataOption) ([]byte, error) {
	var cfg GenerateLockDataConfig

	cfg.Option(opts...)

	pkg, err := loaderOrDefaultLoader(u.cfg.Loader).LoadPackage(ctx, srcPath)
	if err != nil {
		return nil, fmt.Errorf("loading package: %w", err)
	}

	lockImages := make([]manifests.PackageManifestLockImage, len(pkg.Manifest.Spec.Images))
	for i, img := range pkg.Manifest.Spec.Images {
		lockImg, err := u.lockImageFromManifestImage(cfg, img)
		if err != nil {
			return nil, fmt.Errorf("resolving lock image for %q: %w", img.Image, err)
		}

		lockImages[i] = lockImg
	}

	manifestLock := &manifests.PackageManifestLock{
		TypeMeta: metav1.TypeMeta{
			Kind:       packages.PackageManifestLockGroupKind.Kind,
			APIVersion: manifests.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: clockOrDefaultClock(u.cfg.Clock).Now(),
		},
		Spec: manifests.PackageManifestLockSpec{
			Images: lockImages,
		},
	}

	if pkg.ManifestLock != nil && lockSpecsAreEqual(manifestLock.Spec, pkg.ManifestLock.Spec) {
		return nil, ErrLockDataUnchanged
	}
	v1alpha1ManifestLock, err := packages.ToV1Alpha1ManifestLock(manifestLock)
	if err != nil {
		return nil, err
	}

	manifestLockYaml, err := yaml.Marshal(v1alpha1ManifestLock)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling manifest lock file: %w", err)
	}

	return manifestLockYaml, nil
}

var ErrLockDataUnchanged = errors.New("lock data unchanged")

func (u *Update) lockImageFromManifestImage(
	cfg GenerateLockDataConfig, img manifests.PackageManifestImage,
) (manifests.PackageManifestLockImage, error) {
	overriddenImage, err := utils.ImageURLWithOverrideFromEnv(img.Image)
	if err != nil {
		return manifests.PackageManifestLockImage{}, fmt.Errorf("resolving image URL: %w", err)
	}

	digest, err := resolverOrDefaultResolver(u.cfg.Resolver).ResolveDigest(overriddenImage, WithInsecure(cfg.Insecure))
	if err != nil {
		return manifests.PackageManifestLockImage{}, fmt.Errorf("resolving image digest: %w", err)
	}

	return manifests.PackageManifestLockImage{Name: img.Name, Image: img.Image, Digest: digest}, nil
}

func lockSpecsAreEqual(spec manifests.PackageManifestLockSpec, other manifests.PackageManifestLockSpec) bool {
	thisImages := map[string]manifests.PackageManifestLockImage{}
	for _, image := range spec.Images {
		thisImages[image.Name] = image
	}

	otherImages := map[string]manifests.PackageManifestLockImage{}
	for _, image := range other.Images {
		otherImages[image.Name] = image
	}

	if len(thisImages) != len(otherImages) {
		return false
	}

	for name, image := range thisImages {
		otherImage, exists := otherImages[name]
		if !exists || otherImage != image {
			return false
		}
	}

	return true
}

type GenerateLockDataConfig struct {
	Insecure bool
}

func (c *GenerateLockDataConfig) Option(opts ...GenerateLockDataOption) {
	for _, opt := range opts {
		opt.ConfigureGenerateLockData(c)
	}
}

type GenerateLockDataOption interface {
	ConfigureGenerateLockData(*GenerateLockDataConfig)
}

type PackageLoader interface {
	LoadPackage(ctx context.Context, path string) (*packages.Package, error)
}

func NewDefaultPackageLoader(scheme *runtime.Scheme) *DefaultPackageLoader {
	return &DefaultPackageLoader{
		scheme: scheme,
	}
}

type DefaultPackageLoader struct {
	scheme *runtime.Scheme
}

func (l *DefaultPackageLoader) LoadPackage(ctx context.Context, path string) (*packages.Package, error) {
	var rawPkg *packages.RawPackage

	rawPkg, err := packages.FromFolder(ctx, path)
	if err != nil {
		return nil, err
	}

	pkg, err := packages.DefaultStructuralLoader.Load(ctx, rawPkg)
	if err != nil {
		return nil, err
	}

	return pkg, nil
}

type Clock interface {
	Now() metav1.Time
}

type defaultClock struct{}

func (c *defaultClock) Now() metav1.Time {
	return metav1.Now()
}
