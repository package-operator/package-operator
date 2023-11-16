package packagestructure

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
)

type manifestConstraint interface {
	manifests.PackageManifest | manifests.PackageManifestLock | manifests.RepositoryEntry
}

func ManifestFromFile[T manifestConstraint, PT interface {
	runtime.Object
	*T
}](
	_ context.Context, scheme *runtime.Scheme,
	path string, manifestBytes []byte,
) (*T, error) {
	manifest := PT(new(T))
	gvks, _, err := scheme.ObjectKinds(manifest)
	if err != nil {
		return nil, err
	}
	expectedGK := gvks[0].GroupKind()

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
	if gvk.GroupKind() != expectedGK {
		return nil, packagetypes.ViolationError{
			Reason:  packagetypes.ViolationReasonUnknownGVK,
			Details: fmt.Sprintf("GroupKind must be %s, is: %s", expectedGK, gvk.GroupKind()),
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
			Reason:  packagetypes.ViolationReasonUnknownGVK,
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
	if err := scheme.Convert(anyVersionPackageManifest, manifest, nil); err != nil {
		return nil, err
	}

	return manifest, nil
}
