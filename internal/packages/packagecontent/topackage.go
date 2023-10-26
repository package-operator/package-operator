package packagecontent

import (
	"bytes"
	"context"
	"path/filepath"
	"regexp"
	"strings"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"package-operator.run/internal/packages"
)

func PackageFromFiles(ctx context.Context, scheme *runtime.Scheme, files Files) (*Package, error) {
	pkg := &Package{nil, nil, map[string][]unstructured.Unstructured{}}
	var err error
	for path, content := range files {
		switch {
		case strings.HasPrefix(filepath.Base(path), "_"):
			// skip template helper files.
		case !packages.IsYAMLFile(path):
			// skip non YAML files
		case packages.IsManifestFile(path):
			if pkg.PackageManifest, err = processManifestFile(ctx, scheme, pkg.PackageManifest, path, content); err != nil {
				return nil, err
			}
		case packages.IsManifestLockFile(path):
			if pkg.PackageManifestLock != nil {
				err = packages.ViolationError{
					Reason: packages.ViolationReasonPackageManifestLockDuplicated,
					Path:   path,
				}
				return nil, err
			}
			pkg.PackageManifestLock, err = manifestLockFromFile(ctx, scheme, path, content)
			if err != nil {
				return nil, err
			}
		default:
			if err = parseObjects(pkg, path, content); err != nil {
				return nil, err
			}
		}
	}

	if pkg.PackageManifest == nil {
		return nil, packages.ErrManifestNotFound
	}

	return pkg, nil
}

var splitYAMLDocumentsRegEx = regexp.MustCompile(`(?m)^---$`)

func parseObjects(pkg *Package, path string, content []byte) (err error) {
	// Trim empty starting and ending objects
	objects := []unstructured.Unstructured{}

	// Split for every included yaml document.

	for idx, yamlDocument := range splitYAMLDocumentsRegEx.Split(string(bytes.Trim(content, "---\n")), -1) {
		obj := unstructured.Unstructured{}
		if err = yaml.Unmarshal([]byte(yamlDocument), &obj); err != nil {
			err = packages.ViolationError{
				Reason:  packages.ViolationReasonInvalidYAML,
				Details: err.Error(),
				Path:    path,
				Index:   packages.Index(idx),
				Subject: yamlDocument,
			}
			return
		}

		if len(obj.Object) != 0 {
			objects = append(objects, obj)
		}
	}
	if len(objects) != 0 {
		pkg.Objects[path] = objects
	}
	return
}

func processManifestFile(ctx context.Context, scheme *runtime.Scheme, previousManifest *manifestsv1alpha1.PackageManifest, path string, content []byte) (newManifest *manifestsv1alpha1.PackageManifest, err error) {
	if previousManifest != nil {
		return previousManifest, packages.ViolationError{
			Reason: packages.ViolationReasonPackageManifestDuplicated,
			Path:   path,
		}
	}
	return manifestFromFile(ctx, scheme, path, content)
}
