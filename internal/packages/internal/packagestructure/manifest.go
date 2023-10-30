package packagestructure

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
)

func manifestFromFiles(ctx context.Context, scheme *runtime.Scheme, files packagetypes.Files) (*manifests.PackageManifest, error) {
	if bothExtensions(files, packagetypes.PackageManifestFilename) {
		return nil, packagetypes.ViolationError{
			Reason: packagetypes.ViolationReasonPackageManifestDuplicated,
		}
	}
	manifestBytes, manifestPath, manifestFound := getFile(files, packagetypes.PackageManifestFilename)
	if !manifestFound {
		return nil, packagetypes.ErrManifestNotFound
	}
	return manifestFromFile(ctx, scheme, manifestPath, manifestBytes)
}

func manifestFromFile(
	ctx context.Context, scheme *runtime.Scheme,
	path string, manifestBytes []byte,
) (*manifests.PackageManifest, error) {
	return ManifestFromFile[manifests.PackageManifest](ctx, scheme, path, manifestBytes)
}

func manifestLockFromFile(
	ctx context.Context, scheme *runtime.Scheme,
	path string, manifestBytes []byte,
) (*manifests.PackageManifestLock, error) {
	return ManifestFromFile[manifests.PackageManifestLock](ctx, scheme, path, manifestBytes)
}

// Converts the internal version of an PackageManifestLock into it's v1alpha1 representation.
func ToV1Alpha1ManifestLock(in *manifests.PackageManifestLock) (*manifestsv1alpha1.PackageManifestLock, error) {
	out := &manifestsv1alpha1.PackageManifestLock{}
	if err := scheme.Convert(in, out, nil); err != nil {
		return nil, err
	}
	out.SetGroupVersionKind(manifestsv1alpha1.GroupVersion.WithKind("PackageManifestLock"))
	return out, nil
}
