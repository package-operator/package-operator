package packagecontent

import (
	"bytes"
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"package-operator.run/package-operator/internal/packages"
)

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
