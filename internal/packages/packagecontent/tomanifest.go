package packagecontent

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages"
	"package-operator.run/internal/packages/packageadmission"
)

func manifestFromFile(
	ctx context.Context, scheme *runtime.Scheme, fileName string, manifestBytes []byte,
) (*manifestsv1alpha1.PackageManifest, error) {
	// Unmarshal "pre-load" to peek desired GVK.
	var manifestType metav1.TypeMeta
	if err := yaml.Unmarshal(manifestBytes, &manifestType); err != nil {
		return nil, packages.ViolationError{
			Reason:  packages.ViolationReasonInvalidYAML,
			Details: err.Error(),
			Path:    fileName,
		}
	}
	gvk := manifestType.GroupVersionKind()
	if gvk.GroupKind() != packages.PackageManifestGroupKind {
		return nil, packages.ViolationError{
			Reason:  packages.ViolationReasonPackageManifestUnknownGVK,
			Details: fmt.Sprintf("GroupKind must be %s, is: %s", packages.PackageManifestGroupKind, gvk.GroupKind()),
			Path:    fileName,
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

		return nil, packages.ViolationError{
			Reason:  packages.ViolationReasonPackageManifestUnknownGVK,
			Details: fmt.Sprintf("unknown version %s, supported versions: %s", gvk.Version, strings.Join(versions, ", ")),
			Path:    fileName,
		}
	}

	// Unmarshal the given PackageManifest version.
	anyVersionPackageManifest, err := scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(manifestBytes, anyVersionPackageManifest); err != nil {
		return nil, packages.ViolationError{
			Reason:  packages.ViolationReasonInvalidYAML,
			Details: err.Error(),
			Path:    fileName,
		}
	}

	// Default fields in PackageManifest
	scheme.Default(anyVersionPackageManifest)

	// Whatever PackageManifest version we have loaded,
	// we have to convert it to a common/hub version to use throughout the code base:
	manifest := &manifestsv1alpha1.PackageManifest{}
	if err := scheme.Convert(anyVersionPackageManifest, manifest, nil); err != nil {
		return nil, packages.ViolationError{
			Reason:  packages.ViolationReasonPackageManifestConversion,
			Details: err.Error(),
			Path:    fileName,
		}
	}

	fErr, err := packageadmission.ValidatePackageManifest(ctx, scheme, manifest)
	if err != nil {
		return nil, err
	}

	if len(fErr) != 0 {
		return nil, packages.ViolationError{
			Reason:  packages.ViolationReasonPackageManifestInvalid,
			Details: fErr.ToAggregate().Error(),
			Path:    fileName,
		}
	}
	return manifest, nil
}

func manifestLockFromFile(
	ctx context.Context, scheme *runtime.Scheme, fileName string, manifestBytes []byte,
) (*manifestsv1alpha1.PackageManifestLock, error) {
	// Unmarshal "pre-load" to peek desired GVK.
	var manifestType metav1.TypeMeta
	if err := yaml.Unmarshal(manifestBytes, &manifestType); err != nil {
		return nil, packages.ViolationError{
			Reason:  packages.ViolationReasonInvalidYAML,
			Details: err.Error(),
			Path:    fileName,
		}
	}
	gvk := manifestType.GroupVersionKind()
	if gvk.GroupKind() != packages.PackageManifestLockGroupKind {
		return nil, packages.ViolationError{
			Reason:  packages.ViolationReasonPackageManifestLockUnknownGVK,
			Details: fmt.Sprintf("GroupKind must be %s, is: %s", packages.PackageManifestLockGroupKind, gvk.GroupKind()),
			Path:    fileName,
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

		return nil, packages.ViolationError{
			Reason:  packages.ViolationReasonPackageManifestLockUnknownGVK,
			Details: fmt.Sprintf("unknown version %s, supported versions: %s", gvk.Version, strings.Join(versions, ", ")),
			Path:    fileName,
		}
	}

	// Unmarshal the given PackageManifestLock version.
	anyVersionPackageManifest, err := scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(manifestBytes, anyVersionPackageManifest); err != nil {
		return nil, packages.ViolationError{
			Reason:  packages.ViolationReasonInvalidYAML,
			Details: err.Error(),
			Path:    fileName,
		}
	}

	// Default fields in PackageManifest
	scheme.Default(anyVersionPackageManifest)

	// Whatever PackageManifest version we have loaded,
	// we have to convert it to a common/hub version to use throughout the code base:
	manifest := &manifestsv1alpha1.PackageManifestLock{}
	if err := scheme.Convert(anyVersionPackageManifest, manifest, nil); err != nil {
		return nil, packages.ViolationError{
			Reason:  packages.ViolationReasonPackageManifestLockConversion,
			Details: err.Error(),
			Path:    fileName,
		}
	}

	fErr, err := packageadmission.ValidatePackageManifestLock(ctx, manifest)
	if err != nil {
		return nil, err
	}

	if len(fErr) != 0 {
		return nil, packages.ViolationError{
			Reason:  packages.ViolationReasonPackageManifestLockInvalid,
			Details: fErr.ToAggregate().Error(),
			Path:    fileName,
		}
	}
	return manifest, nil
}
