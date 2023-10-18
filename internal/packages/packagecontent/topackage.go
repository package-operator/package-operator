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

// this regex matches a path inside the "components/" folder consisting of:
//   - the component directory, whose name must match "RFC 1123 Label Names"
//     see https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names
//   - the rest of the path (separated from the first element by a "/") which will be processed as separate package
var componentFileRE = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)/(.+)$`)

func PackageFromFiles(ctx context.Context, scheme *runtime.Scheme, files Files, component string) (*Package, error) {
	pkgMap, err := AllPackagesFromFiles(ctx, scheme, files, component)
	if err != nil {
		return nil, err
	}
	return ExtractComponentPackage(pkgMap, component)
}

func parseComponentsFiles(files Files, filesMap map[string]Files) (err error) {
	paths := make([]string, 0, len(files))
	for key := range files {
		paths = append(paths, key)
	}
	sort.Strings(paths)

	for _, path := range paths {
		componentName, componentPath, err := getComponentNameAndPath(path)
		if err != nil {
			return err
		}
		if _, exists := filesMap[componentName]; !exists {
			filesMap[componentName] = Files{}
		}
		filesMap[componentName][componentPath] = files[path]
	}
	return
}

func AllPackagesFromFiles(ctx context.Context, scheme *runtime.Scheme, files Files, component string) (map[string]*Package, error) {
	componentsEnabled, err := areComponentsEnabled(ctx, scheme, files)
	if err != nil {
		return nil, err
	}

	filesMap := map[string]Files{}
	if !componentsEnabled {
		if component != "" {
			return nil, packages.ViolationError{Reason: packages.ViolationReasonComponentsNotEnabled}
		}
		filesMap[""] = files
	} else if err = parseComponentsFiles(files, filesMap); err != nil {
		return nil, err
	}

	_, exists := filesMap[component]
	if !exists {
		return nil, packages.ViolationError{Reason: packages.ViolationReasonComponentNotFound, Component: component}
	}

	pkgMap := map[string]*Package{}
	for componentName, componentFiles := range filesMap {
		pkg, err := buildPackageFromFiles(ctx, scheme, componentFiles, componentName)
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

func buildPackageFromFiles(ctx context.Context, scheme *runtime.Scheme, files Files, component string) (*Package, error) {
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

			if isMultiComponent(pkg.PackageManifest) && component != "" {
				// when building a component, do not allow nested components
				err = packages.ViolationError{
					Reason:    packages.ViolationReasonNestedMultiComponentPkg,
					Path:      path,
					Component: component,
				}
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

func processManifestFile(ctx context.Context, scheme *runtime.Scheme, previousManifest *manifestsv1alpha1.PackageManifest, path string, content []byte) (newManifest *manifestsv1alpha1.PackageManifest, err error) {
	if previousManifest != nil {
		return previousManifest, packages.ViolationError{
			Reason: packages.ViolationReasonPackageManifestDuplicated,
			Path:   path,
		}
	}
	return manifestFromFile(ctx, scheme, path, content)
}

func isMultiComponent(manifest *manifestsv1alpha1.PackageManifest) bool {
	return manifest != nil && manifest.Spec.Components != nil
}

func areComponentsEnabled(ctx context.Context, scheme *runtime.Scheme, files Files) (result bool, err error) {
	var manifest *manifestsv1alpha1.PackageManifest
	for path, content := range files {
		if !strings.HasPrefix(path, "components/") && packages.IsManifestFile(path) {
			if manifest, err = processManifestFile(ctx, scheme, manifest, path, content); err != nil {
				return false, err
			}
			return isMultiComponent(manifest), nil
		}
	}
	return false, packages.ErrManifestNotFound
}

func getComponentNameAndPath(path string) (componentName string, componentPath string, err error) {
	if !strings.HasPrefix(path, "components/") {
		return "", path, nil
	}

	if matches := componentFileRE.FindStringSubmatch(path[11:]); len(matches) == 4 {
		return matches[1], matches[3], nil
	}

	if strings.Count(path, "/") > 1 {
		// invalid component name
		return "", "", packages.ViolationError{
			Reason: packages.ViolationReasonInvalidComponentPath,
			Path:   path,
		}
	}

	if !strings.HasPrefix(path, "components/.") {
		// not a dot file in the components dir
		return "", "", packages.ViolationError{
			Reason: packages.ViolationReasonInvalidFileInComponentsDir,
			Path:   path,
		}
	}

	return "", path, nil
}
