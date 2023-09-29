package packagecontent

import (
	"bytes"
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages"
)

func inRootDir(path string) bool {
	return !strings.ContainsRune(path, '/')
}

func isMultiComponent(manifest *v1alpha1.PackageManifest) bool {
	return manifest != nil && manifest.Spec.Component != nil
}

func parseRootManifest(ctx context.Context, scheme *runtime.Scheme, files Files) (*v1alpha1.PackageManifest, error) {
	var (
		manifest *v1alpha1.PackageManifest
		err      error
	)

	for path, content := range files {
		if inRootDir(path) && packages.IsManifestFile(path) {
			manifest, err = manifestFromFile(ctx, scheme, path, content)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	if manifest == nil {
		return nil, packages.ErrManifestNotFound
	}
	return manifest, nil
}

func parseObjects(pkg *Package, path string, content []byte) (err error) {
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
	return
}

func parseSimplePackage(ctx context.Context, scheme *runtime.Scheme, files Files) (*Package, error) {
	pkg := &Package{
		PackageManifest:     nil,
		PackageManifestLock: nil,
		Objects:             map[string][]unstructured.Unstructured{},
	}
	var err error

	for path, content := range files {
		switch {
		case !packages.IsYAMLFile(path):
			// skip non YAML files
			continue
		case packages.IsManifestFile(path):
			if pkg.PackageManifest != nil {
				err = packages.ViolationError{
					Reason: packages.ViolationReasonPackageManifestDuplicated,
					Path:   path,
				}
				return nil, err
			}

			pkg.PackageManifest, err = manifestFromFile(ctx, scheme, path, content)
			if err != nil {
				return nil, err
			}

			if isMultiComponent(pkg.PackageManifest) {
				// nesting multi-component packages is not allowed
				err = packages.ViolationError{
					Reason: packages.ViolationReasonNestedMultiComponentPkg,
					Path:   path,
				}
			}
			continue
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
			continue
		}

		err = parseObjects(pkg, path, content)
		if err != nil {
			return nil, err
		}
	}

	if pkg.PackageManifest == nil {
		err = packages.ErrManifestNotFound
		return nil, err
	}

	return pkg, nil
}

func stripDir(path, dir string) string {
	return ""
}

func inComponentsDir(path string) (bool, error) {
	return strings.HasPrefix(path, "components/") && path != "components/", nil
}

func getComponentsDirFiles(files Files) (Files, error) {
	return nil, nil
}

func parseComponentsDir() error {
	return nil
}

func splitFiles(files Files) error {
	return nil
}

func parseMultiComponentPackage(ctx context.Context, scheme *runtime.Scheme, files Files) (*Package, error) {
	return nil, nil
}

func PackageFromFiles(ctx context.Context, scheme *runtime.Scheme, files Files) (*Package, error) {
	manifest, err := parseRootManifest(ctx, scheme, files)
	if err != nil {
		return nil, err
	}

	if isMultiComponent(manifest) {
		return parseMultiComponentPackage(ctx, scheme, files)
	}
	return parseSimplePackage(ctx, scheme, files)
}
