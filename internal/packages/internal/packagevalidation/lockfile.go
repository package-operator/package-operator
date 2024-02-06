package packagevalidation

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
	"package-operator.run/internal/utils"
)

// Validates that images referenced in the lockfile are still present in the registry.
type LockfileDigestLookupValidator struct {
	// Options passed to crane when looking up the digest.
	CraneOptions []crane.Option

	digestLookupFn func(ref string, opt ...crane.Option) (string, error)
}

func (v *LockfileDigestLookupValidator) ValidatePackage(
	_ context.Context, pkg *packagetypes.Package,
) error {
	if pkg.ManifestLock == nil {
		return nil
	}
	digestLookup := v.digestLookupFn
	if digestLookup == nil {
		digestLookup = crane.Digest
	}

	for _, image := range pkg.ManifestLock.Spec.Images {
		overriddenImage, err := utils.ImageURLWithOverrideFromEnv(image.Image)
		if err != nil {
			return fmt.Errorf("%w: can't parse image %q reference %q", err, image.Name, image.Image)
		}
		ref, err := name.ParseReference(overriddenImage)
		if err != nil {
			return fmt.Errorf("%w: can't parse image %q reference %q", err, image.Name, image.Image)
		}
		digestRef := ref.Context().Digest(image.Digest)
		if _, err := digestLookup(digestRef.String(), v.CraneOptions...); err != nil {
			return fmt.Errorf("%w: image %q digest error (%q)", err, image.Name, image.Digest)
		}
	}
	return nil
}

// Validates that the PackageManifestLock is consistent with PackageManifest.
type LockfileConsistencyValidator struct{}

func (v *LockfileConsistencyValidator) ValidatePackage(
	_ context.Context, pkg *packagetypes.Package,
) error {
	lockfile := packagetypes.PackageManifestLockFilename + ".yaml"

	if pkg.ManifestLock == nil {
		if len(pkg.Manifest.Spec.Images) > 0 {
			return packagetypes.ViolationError{
				Reason: packagetypes.ViolationReasonLockfileMissing,
				Path:   lockfile,
			}
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
			return packagetypes.ViolationError{
				Reason:  packagetypes.ViolationReasonLockfileMissing,
				Details: imageName,
			}
		}

		if image.Image != lockImage.Image {
			return packagetypes.ViolationError{
				Reason:  packagetypes.ViolationReasonImageDifferentToLockfile,
				Details: fmt.Sprintf("%q: %q vs %q", imageName, image.Image, lockImage.Image),
			}
		}
	}

	// check that all the images in lock file exists in manifest files too (which ensures manifest images == lock images)
	for imageName := range pkgLockImages {
		_, existsInManifest := pkgImages[imageName]
		if !existsInManifest {
			return packagetypes.ViolationError{
				Reason:  packagetypes.ViolationReasonImageDifferentToLockfile,
				Details: imageName,
			}
		}
	}

	return nil
}
