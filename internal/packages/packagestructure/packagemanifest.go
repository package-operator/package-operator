package packagestructure

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/packages/packagebytes"
)

// PackageManifestLoader implements parsing of PackageManifest object from bytes or files.
// This loader can handle GVK mismatches gracefully for forward and backwards compatibility.
type PackageManifestLoader struct {
	scheme *runtime.Scheme
}

func NewPackageManifestLoader(scheme *runtime.Scheme) *PackageManifestLoader {
	return &PackageManifestLoader{
		scheme: scheme,
	}
}

// Load PackageManifest from the given FileMap.
func (l *PackageManifestLoader) FromFileMap(ctx context.Context, fm packagebytes.FileMap) (
	*manifestsv1alpha1.PackageManifest, error,
) {
	verboseLog := logr.FromContextOrDiscard(ctx).V(1)

	var (
		manifestBytes           []byte
		packageManifestFileName string
	)
	for i := range packages.PackageManifestFileNames {
		packageManifestFileName = packages.PackageManifestFileNames[i]

		var ok bool
		if manifestBytes, ok = fm[packageManifestFileName]; ok {
			verboseLog.Info("found manifest", "path", packageManifestFileName)
			break
		}
	}

	if manifestBytes == nil {
		return nil, packages.NewInvalidError(packages.Violation{
			Reason:  packages.ViolationReasonPackageManifestNotFound,
			Details: "searched at " + strings.Join(packages.PackageManifestFileNames, ","),
		})
	}

	return l.manifestFromBytes(ctx, packageManifestFileName, manifestBytes)
}

// Load PackageManifest from bytes.
// Does not support multi-document yaml byte input.
func (l *PackageManifestLoader) manifestFromBytes(
	ctx context.Context, fileName string, manifestBytes []byte,
) (
	*manifestsv1alpha1.PackageManifest, error,
) {
	// Unmarshal "pre-load" to peek desired GVK.
	var manifestType metav1.TypeMeta
	if err := yaml.Unmarshal(manifestBytes, &manifestType); err != nil {
		return nil, packages.NewInvalidError(packages.Violation{
			Reason:   packages.ViolationReasonInvalidYAML,
			Details:  err.Error(),
			Location: &packages.ViolationLocation{Path: fileName},
		})
	}
	gvk := manifestType.GroupVersionKind()
	if gvk.GroupKind() != packages.PackageManifestGroupKind {
		return nil, packages.NewInvalidError(packages.Violation{
			Reason:   packages.ViolationReasonPackageManifestUnknownGVK,
			Details:  fmt.Sprintf("GroupKind must be %s, is: %s", packages.PackageManifestGroupKind, gvk.GroupKind()),
			Location: &packages.ViolationLocation{Path: fileName},
		})
	}

	if !l.scheme.Recognizes(gvk) {
		// GroupKind is ok, so the version is not recognized.
		// Either the Package we are trying is very old and support was dropped or
		// Package is build for a newer PKO version.
		groupVersions := l.scheme.VersionsForGroupKind(gvk.GroupKind())
		versions := make([]string, len(groupVersions))
		for i := range groupVersions {
			versions[i] = groupVersions[i].Version
		}

		return nil, packages.NewInvalidError(packages.Violation{
			Reason:   packages.ViolationReasonPackageManifestUnknownGVK,
			Details:  fmt.Sprintf("unknown version %s, supported versions: %s", gvk.Version, strings.Join(versions, ", ")),
			Location: &packages.ViolationLocation{Path: fileName},
		})
	}

	// Unmarshal the given PackageManifest version.
	anyVersionPackageManifest, err := l.scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(manifestBytes, anyVersionPackageManifest); err != nil {
		return nil, packages.NewInvalidError(packages.Violation{
			Reason:   packages.ViolationReasonInvalidYAML,
			Details:  err.Error(),
			Location: &packages.ViolationLocation{Path: fileName},
		})
	}

	// Default fields in PackageManifest
	l.scheme.Default(anyVersionPackageManifest)

	// Whatever PackageManifest version we have loaded,
	// we have to convert it to a common/hub version to use throughout the code base:
	manifest := &manifestsv1alpha1.PackageManifest{}
	if err := l.scheme.Convert(anyVersionPackageManifest, manifest, nil); err != nil {
		return nil, packages.NewInvalidError(packages.Violation{
			Reason:   packages.ViolationReasonPackageManifestConversion,
			Details:  err.Error(),
			Location: &packages.ViolationLocation{Path: fileName},
		})
	}

	if err := manifest.Validate(); err != nil {
		return nil, packages.NewInvalidError(packages.Violation{
			Reason:   packages.ViolationReasonPackageManifestInvalid,
			Details:  err.Error(),
			Location: &packages.ViolationLocation{Path: fileName},
		})
	}
	return manifest, nil
}
