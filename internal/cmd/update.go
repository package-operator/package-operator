package cmd

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/packages/packageimport"
	"package-operator.run/package-operator/internal/packages/packageloader"
	"package-operator.run/package-operator/internal/utils"
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
		c.Loader = &defaultPackageLoader{}
	}

	if c.Resolver == nil {
		c.Resolver = &defaultDigestResolver{}
	}
}

type UpdateOption interface {
	ConfigureUpdate(*UpdateConfig)
}

func (u *Update) GenerateLockData(ctx context.Context, srcPath string) ([]byte, error) {
	pkg, err := u.cfg.Loader.LoadPackage(ctx, srcPath)
	if err != nil {
		return nil, fmt.Errorf("loading package: %w", err)
	}

	var lockImages []v1alpha1.PackageManifestLockImage
	for _, img := range pkg.PackageManifest.Spec.Images {
		lockImg, err := u.lockImageFromManifestImage(img)
		if err != nil {
			return nil, fmt.Errorf("resolving lock image for %q: %w", img.Image, err)
		}

		lockImages = append(lockImages, lockImg)
	}

	manifestLock := &v1alpha1.PackageManifestLock{
		TypeMeta: v1.TypeMeta{
			Kind:       packages.PackageManifestLockGroupKind.Kind,
			APIVersion: v1alpha1.GroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			CreationTimestamp: u.cfg.Clock.Now(),
		},
		Spec: v1alpha1.PackageManifestLockSpec{
			Images: lockImages,
		},
	}

	if pkg.PackageManifestLock != nil && lockSpecsAreEqual(manifestLock.Spec, pkg.PackageManifestLock.Spec) {
		return nil, nil
	}

	manifestLockYaml, err := yaml.Marshal(manifestLock)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling manifest lock file: %w", err)
	}

	return manifestLockYaml, nil
}

func (u *Update) lockImageFromManifestImage(img v1alpha1.PackageManifestImage) (v1alpha1.PackageManifestLockImage, error) {
	overriddenImage, err := utils.ImageURLWithOverrideFromEnv(img.Image)
	if err != nil {
		return v1alpha1.PackageManifestLockImage{}, fmt.Errorf("resolving image URL: %w", err)
	}

	digest, err := u.cfg.Resolver.ResolveDigest(overriddenImage)
	if err != nil {
		return v1alpha1.PackageManifestLockImage{}, fmt.Errorf("resolving image digest: %w", err)
	}

	return v1alpha1.PackageManifestLockImage{
		Name:   img.Name,
		Image:  img.Image,
		Digest: digest,
	}, nil
}

func lockSpecsAreEqual(spec v1alpha1.PackageManifestLockSpec, other v1alpha1.PackageManifestLockSpec) bool {
	thisImages := map[string]v1alpha1.PackageManifestLockImage{}
	for _, image := range spec.Images {
		thisImages[image.Name] = image
	}

	otherImages := map[string]v1alpha1.PackageManifestLockImage{}
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

type PackageLoader interface {
	LoadPackage(ctx context.Context, path string) (*packagecontent.Package, error)
}

type defaultPackageLoader struct{}

func (l *defaultPackageLoader) LoadPackage(ctx context.Context, path string) (*packagecontent.Package, error) {
	var fileMap packagecontent.Files

	fileMap, err := packageimport.Folder(ctx, path)
	if err != nil {
		return nil, err
	}

	scheme, err := NewScheme()
	if err != nil {
		return nil, err
	}

	pkg, err := packageloader.New(scheme).FromFiles(ctx, fileMap)
	if err != nil {
		return nil, err
	}

	return pkg, nil
}

type Clock interface {
	Now() v1.Time
}

type defaultClock struct{}

func (c *defaultClock) Now() v1.Time {
	return v1.Now()
}
