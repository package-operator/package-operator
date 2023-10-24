package packagestructure

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
)

func manifestFromFile(
	_ context.Context, scheme *runtime.Scheme,
	path string, manifestBytes []byte,
) (*manifests.PackageManifest, error) {
	// Unmarshal "pre-load" to peek desired GVK.
	var manifestType metav1.TypeMeta
	if err := yaml.Unmarshal(manifestBytes, &manifestType); err != nil {
		return nil, packagetypes.ViolationError{
			Reason:  packagetypes.ViolationReasonInvalidYAML,
			Details: err.Error(),
			Path:    path,
		}
	}
	gvk := manifestType.GroupVersionKind()
	if gvk.GroupKind() != packagetypes.PackageManifestGroupKind {
		return nil, packagetypes.ViolationError{
			Reason:  packagetypes.ViolationReasonPackageManifestUnknownGVK,
			Details: fmt.Sprintf("GroupKind must be %s, is: %s", packagetypes.PackageManifestGroupKind, gvk.GroupKind()),
			Path:    path,
		}
	}

	if !scheme.Recognizes(gvk) {
		// GroupKind is ok, so the version is not recognized.
		// Either the Package we are trying is very old and support was dropped or
		// Package is build for a newer PKO version.
		groupVersions := scheme.VersionsForGroupKind(gvk.GroupKind())
		versions := make([]string, len(groupVersions))
		for i := range groupVersions {
			versions[i] = groupVersions[i].Version
		}

		return nil, packagetypes.ViolationError{
			Reason:  packagetypes.ViolationReasonPackageManifestUnknownGVK,
			Details: fmt.Sprintf("unknown version %s, supported versions: %s", gvk.Version, strings.Join(versions, ", ")),
			Path:    path,
		}
	}

	// Unmarshal the given PackageManifest version.
	anyVersionPackageManifest, err := scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(manifestBytes, anyVersionPackageManifest); err != nil {
		return nil, packagetypes.ViolationError{
			Reason:  packagetypes.ViolationReasonInvalidYAML,
			Details: err.Error(),
			Path:    path,
		}
	}

	// Default fields in PackageManifest
	scheme.Default(anyVersionPackageManifest)

	// Whatever PackageManifest version we have loaded,
	// we have to convert it to a common/hub version to use throughout the code base:
	manifest := &manifests.PackageManifest{}
	if err := scheme.Convert(anyVersionPackageManifest, manifest, nil); err != nil {
		return nil, packagetypes.ViolationError{
			Reason:  packagetypes.ViolationReasonPackageManifestConversion,
			Details: err.Error(),
			Path:    path,
		}
	}

	return manifest, nil
}

func manifestLockFromFile(
	_ context.Context, scheme *runtime.Scheme,
	path string, manifestBytes []byte,
) (*manifests.PackageManifestLock, error) {
	// Unmarshal "pre-load" to peek desired GVK.
	var manifestType metav1.TypeMeta
	if err := yaml.Unmarshal(manifestBytes, &manifestType); err != nil {
		return nil, packagetypes.ViolationError{
			Reason:  packagetypes.ViolationReasonInvalidYAML,
			Details: err.Error(),
			Path:    path,
		}
	}
	gvk := manifestType.GroupVersionKind()
	if gvk.GroupKind() != packagetypes.PackageManifestLockGroupKind {
		return nil, packagetypes.ViolationError{
			Reason:  packagetypes.ViolationReasonPackageManifestLockUnknownGVK,
			Details: fmt.Sprintf("GroupKind must be %s, is: %s", packagetypes.PackageManifestLockGroupKind, gvk.GroupKind()),
			Path:    path,
		}
	}

	if !scheme.Recognizes(gvk) {
		// GroupKind is ok, so the version is not recognized.
		// Either the Package we are trying is very old and support was dropped or
		// Package is build for a newer PKO version.
		groupVersions := scheme.VersionsForGroupKind(gvk.GroupKind())
		versions := make([]string, len(groupVersions))
		for i := range groupVersions {
			versions[i] = groupVersions[i].Version
		}

		return nil, packagetypes.ViolationError{
			Reason:  packagetypes.ViolationReasonPackageManifestLockUnknownGVK,
			Details: fmt.Sprintf("unknown version %s, supported versions: %s", gvk.Version, strings.Join(versions, ", ")),
			Path:    path,
		}
	}

	// Unmarshal the given PackageManifestLock version.
	anyVersionPackageManifest, err := scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(manifestBytes, anyVersionPackageManifest); err != nil {
		return nil, packagetypes.ViolationError{
			Reason:  packagetypes.ViolationReasonInvalidYAML,
			Details: err.Error(),
			Path:    path,
		}
	}

	// Default fields in PackageManifest
	scheme.Default(anyVersionPackageManifest)

	// Whatever PackageManifest version we have loaded,
	// we have to convert it to a common/hub version to use throughout the code base:
	manifest := &manifests.PackageManifestLock{}
	if err := scheme.Convert(anyVersionPackageManifest, manifest, nil); err != nil {
		return nil, packagetypes.ViolationError{
			Reason:  packagetypes.ViolationReasonPackageManifestLockConversion,
			Details: err.Error(),
			Path:    path,
		}
	}

	return manifest, nil
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
