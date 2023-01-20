package packagecontent

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages"
)

func manifestFromFile(
	ctx context.Context, scheme *runtime.Scheme, fileName string, manifestBytes []byte) (*manifestsv1alpha1.PackageManifest, error) {
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

	if !scheme.Recognizes(gvk) {
		// GroupKind is ok, so the version is not recognized.
		// Either the Package we are trying is very old and support was dropped or
		// Package is build for a newer PKO version.
		groupVersions := scheme.VersionsForGroupKind(gvk.GroupKind())
		versions := make([]string, len(groupVersions))
		for i := range groupVersions {
			versions[i] = groupVersions[i].Version
		}

		violation := packages.Violation{
			Reason:   packages.ViolationReasonPackageManifestUnknownGVK,
			Details:  fmt.Sprintf("unknown version %s, supported versions: %s", gvk.Version, strings.Join(versions, ", ")),
			Location: &packages.ViolationLocation{Path: fileName},
		}
		return nil, packages.NewInvalidError(violation)
	}

	// Unmarshal the given PackageManifest version.
	anyVersionPackageManifest, err := scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(manifestBytes, anyVersionPackageManifest); err != nil {
		violation := packages.Violation{
			Reason:   packages.ViolationReasonInvalidYAML,
			Details:  err.Error(),
			Location: &packages.ViolationLocation{Path: fileName},
		}
		return nil, packages.NewInvalidError(violation)
	}

	// Default fields in PackageManifest
	scheme.Default(anyVersionPackageManifest)

	// Whatever PackageManifest version we have loaded,
	// we have to convert it to a common/hub version to use throughout the code base:
	manifest := &manifestsv1alpha1.PackageManifest{}
	if err := scheme.Convert(anyVersionPackageManifest, manifest, nil); err != nil {
		violation := packages.Violation{
			Reason:   packages.ViolationReasonPackageManifestConversion,
			Details:  err.Error(),
			Location: &packages.ViolationLocation{Path: fileName},
		}
		return nil, packages.NewInvalidError(violation)
	}

	if err := manifest.Validate(); err != nil {
		violation := packages.Violation{
			Reason:   packages.ViolationReasonPackageManifestInvalid,
			Details:  err.Error(),
			Location: &packages.ViolationLocation{Path: fileName},
		}
		return nil, packages.NewInvalidError(violation)
	}
	return manifest, nil
}

func PackageFromFiles(ctx context.Context, scheme *runtime.Scheme, files Files) (pkg *Package, err error) {
	pkg = &Package{nil, map[string][]unstructured.Unstructured{}}
	for path, content := range files {
		switch {
		case !packages.IsYAMLFile(path):
			// skip non YAML files
			continue
		case packages.IsManifestFile(path):
			if pkg.PackageManifest != nil {
				err = packages.NewInvalidError(packages.Violation{
					Reason:   packages.ViolationReasonPackageManifestDuplicated,
					Location: &packages.ViolationLocation{Path: path},
				})

				return
			}
			pkg.PackageManifest, err = manifestFromFile(ctx, scheme, path, content)
			if err != nil {
				return nil, err
			}

			continue
		}

		// Trim empty starting and ending objects
		objects := []unstructured.Unstructured{}

		// Split for every included yaml document.
		for i, yamlDocument := range bytes.Split(bytes.Trim(content, "---\n"), []byte("---\n")) {
			obj := unstructured.Unstructured{}
			if err = yaml.Unmarshal(yamlDocument, &obj); err != nil {
				err = packages.NewInvalidError(packages.Violation{
					Reason:   packages.ViolationReasonInvalidYAML,
					Details:  err.Error(),
					Location: &packages.ViolationLocation{Path: path, DocumentIndex: pointer.Int(i)},
				})

				return
			}

			if len(obj.Object) != 0 {
				objects = append(objects, obj)
			}
		}
		if len(objects) != 0 {
			pkg.Objects[path] = objects
		}
	}

	if pkg.PackageManifest == nil {
		violation := packages.Violation{
			Reason:  packages.ViolationReasonPackageManifestNotFound,
			Details: "searched at " + strings.Join(packages.PackageManifestFileNames, ","),
		}

		err = packages.NewInvalidError(violation)
		return
	}

	return
}
