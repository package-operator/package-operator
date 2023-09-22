package packagecontent

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"package-operator.run/internal/packages"
)

func PackageFromFiles(ctx context.Context, scheme *runtime.Scheme, files Files) (pkg *Package, err error) {
	pkg = &Package{nil, nil, map[string][]unstructured.Unstructured{}}
	for path, content := range files {
		switch {
		case !packages.IsYAMLFile(path) ||
			strings.HasPrefix(filepath.Base(path), "_"):
			// skip non YAML files
			continue

		case packages.IsManifestFile(path):
			if pkg.PackageManifest != nil {
				err = packages.ViolationError{
					Reason: packages.ViolationReasonPackageManifestDuplicated,
					Path:   path,
				}

				return
			}
			pkg.PackageManifest, err = manifestFromFile(ctx, scheme, path, content)
			if err != nil {
				return nil, err
			}

			continue
		case packages.IsManifestLockFile(path):
			if pkg.PackageManifestLock != nil {
				err = packages.ViolationError{
					Reason: packages.ViolationReasonPackageManifestLockDuplicated,
					Path:   path,
				}

				return
			}
			pkg.PackageManifestLock, err = manifestLockFromFile(ctx, scheme, path, content)
			if err != nil {
				return nil, err
			}

			continue
		}

		// Trim empty starting and ending objects
		objects := []unstructured.Unstructured{}

		// Split for every included yaml document.
		for idx, yamlDocument := range bytes.Split(bytes.Trim(content, "---\n"), []byte("---\n")) {
			obj := unstructured.Unstructured{}
			if err = yaml.Unmarshal(yamlDocument, &obj); err != nil {
				err = packages.ViolationError{
					Reason:  packages.ViolationReasonInvalidYAML,
					Details: err.Error(),
					Path:    path,
					Index:   packages.Index(idx),
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
	}

	if pkg.PackageManifest == nil {
		err = packages.ErrManifestNotFound
		return
	}

	return
}
