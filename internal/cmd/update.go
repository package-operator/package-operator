package cmd

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages"
	"package-operator.run/internal/packages/resolving/resolvebuild"
	"package-operator.run/internal/utils"
)

func NewUpdate(opts ...UpdateOption) *Update {
	var cfg UpdateConfig

	cfg.Option(opts...)
	cfg.Default()

	return &Update{
		cfg: cfg,
	}
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

func (c *UpdateConfig) Default() {
	if c.Log.GetSink() == nil {
		c.Log = logr.Discard()
	}

	if c.Clock == nil {
		c.Clock = &defaultClock{}
	}

	if c.Loader == nil {
		c.Loader = NewDefaultPackageLoader(runtime.NewScheme())
	}

	if c.Resolver == nil {
		c.Resolver = &defaultDigestResolver{}
	}
}

type UpdateOption interface {
	ConfigureUpdate(*UpdateConfig)
}

func (u *Update) GenerateLockData(ctx context.Context, srcPath string, opts ...GenerateLockDataOption) (data []byte, err error) {
	var cfg GenerateLockDataConfig

	cfg.Option(opts...)

	pkg, err := u.cfg.Loader.LoadPackage(ctx, srcPath)
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

	r := resolvebuild.Resolver{}
	lcks, err := r.ResolveBuild(ctx, pkg.Manifest)
	if err != nil {
		return nil, err
	}

	manifestLock := &manifests.PackageManifestLock{
		TypeMeta: metav1.TypeMeta{
			Kind:       packages.PackageManifestLockGroupKind.Kind,
			APIVersion: manifests.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: u.cfg.Clock.Now(),
		},
		Spec: manifests.PackageManifestLockSpec{
			Images:       lockImages,
			Dependencies: lcks,
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

func (u *Update) lockImageFromManifestImage(cfg GenerateLockDataConfig, img manifests.PackageManifestImage) (manifests.PackageManifestLockImage, error) {
	overriddenImage, err := utils.ImageURLWithOverrideFromEnv(img.Image)
	if err != nil {
		return manifests.PackageManifestLockImage{}, fmt.Errorf("resolving image URL: %w", err)
	}

	digest, err := u.cfg.Resolver.ResolveDigest(overriddenImage, WithInsecure(cfg.Insecure))
	if err != nil {
		return manifests.PackageManifestLockImage{}, fmt.Errorf("resolving image digest: %w", err)
	}

	return manifests.PackageManifestLockImage{
		Name:   img.Name,
		Image:  img.Image,
		Digest: digest,
	}, nil
}

func lockSpecsAreEqual(spec manifests.PackageManifestLockSpec, other manifests.PackageManifestLockSpec) bool {
	return reflect.DeepEqual(spec, other)
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
