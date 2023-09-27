package packagecontent

import (
	"bytes"
	"context"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"package-operator.run/internal/packages"
)

var componentFileRE = regexp.MustCompile("^([a-z][a-z0-9-]{0,62})/(.+)$")

func PackageFromFiles(ctx context.Context, scheme *runtime.Scheme, files Files, component string) (*Package, error) {
	pkgMap, err := AllPackagesFromFiles(ctx, scheme, files, component)
	if err != nil {
		return nil, err
	}
	return ExtractComponentPackage(pkgMap, component)
}

func AllPackagesFromFiles(ctx context.Context, scheme *runtime.Scheme, files Files, component string) (map[string]*Package, error) {
	componentsEnabled, err := areComponentsEnabled(ctx, scheme, files)
	if err != nil {
		return nil, err
	}

	filesMap := make(map[string]Files)
	if !componentsEnabled {
		if component != "" {
			return nil, packages.ViolationError{Reason: packages.ViolationReasonComponentsNotEnabled}
		}
		filesMap[""] = files
	} else {
		paths := make([]string, 0, len(files))
		for key := range files {
			paths = append(paths, key)
		}
		sort.Strings(paths)

		for _, path := range paths {
			componentName, componentPath, err := getComponentNameAndPath(path)
			if err != nil {
				return nil, err
			}
			if _, exists := filesMap[componentName]; !exists {
				filesMap[componentName] = Files{}
			}
			filesMap[componentName][componentPath] = files[path]
		}
	}

	_, exists := filesMap[component]
	if !exists {
		return nil, packages.ViolationError{Reason: packages.ViolationReasonComponentNotFound, Component: component}
	}

	pkgMap := make(map[string]*Package)
	for componentName, componentFiles := range filesMap {
		pkg, err := buildPackageFromFiles(ctx, scheme, componentFiles)
		if err != nil {
			return nil, err
		}
		pkgMap[componentName] = pkg
	}

	return pkgMap, nil
}

func ExtractComponentPackage(pkgMap map[string]*Package, component string) (*Package, error) {
	pkg, exists := pkgMap[component]
	if !exists {
		return nil, packages.ViolationError{Reason: packages.ViolationReasonComponentNotFound, Component: component}
	}
	return pkg, nil
}

func buildPackageFromFiles(ctx context.Context, scheme *runtime.Scheme, files Files) (pkg *Package, err error) {
	pkg = &Package{nil, nil, map[string][]unstructured.Unstructured{}}
	for path, content := range files {
		switch {
		case strings.HasPrefix(filepath.Base(path), "_"):
			// skip template helper files.
			continue
		case !packages.IsYAMLFile(path):
			// skip non YAML files
			continue

		case packages.IsManifestFile(path):
			if pkg.PackageManifest, err = processManifestFile(ctx, scheme, pkg.PackageManifest, path, content); err != nil {
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

func processManifestFile(ctx context.Context, scheme *runtime.Scheme, previousManifest *manifestsv1alpha1.PackageManifest, path string, content []byte) (newManifest *manifestsv1alpha1.PackageManifest, err error) {
	if previousManifest != nil {
		return previousManifest, packages.ViolationError{
			Reason: packages.ViolationReasonPackageManifestDuplicated,
			Path:   path,
		}
	}
	return manifestFromFile(ctx, scheme, path, content)
}

func areComponentsEnabled(ctx context.Context, scheme *runtime.Scheme, files Files) (result bool, err error) {
	var manifest *manifestsv1alpha1.PackageManifest
	for path, content := range files {
		if packages.IsManifestFile(path) {
			if manifest, err = processManifestFile(ctx, scheme, manifest, path, content); err != nil {
				return false, err
			}
		}
	}
	if manifest == nil {
		return false, packages.ErrManifestNotFound
	}
	return manifest.Spec.Components != nil, nil
}

func getComponentNameAndPath(path string) (componentName string, componentPath string, err error) {
	if !strings.HasPrefix(path, "components/") {
		return "", path, nil
	}
	if matches := componentFileRE.FindStringSubmatch(path[11:]); len(matches) == 3 {
		return matches[1], matches[2], nil
	}
	return "", "", packages.ViolationError{
		Reason: packages.ViolationReasonInvalidComponentPath,
		Path:   path,
	}
}
