package packagecontent

import (
	"context"
	"regexp"
	"sort"
	"strings"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"

	"package-operator.run/internal/packages"
)

// this regex matches a path inside the "components/" folder consisting of:
//   - the component directory, whose name must match "RFC 1123 Label Names"
//     see https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names
//   - the rest of the path (separated from the first element by a "/") which will be processed as separate package
var componentFileRE = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)/(.+)$`)

func SplitFilesByComponent(ctx context.Context, scheme *runtime.Scheme, files Files, component string) (map[string]Files, error) {
	componentsEnabled, err := areComponentsEnabled(ctx, scheme, files)
	if err != nil {
		return nil, err
	}

	if !componentsEnabled {
		if component != "" {
			return nil, packages.ViolationError{Reason: packages.ViolationReasonComponentsNotEnabled}
		}
		return map[string]Files{"": files}, nil
	}

	filesMap, err := doSplitFilesByComponent(files)
	if err != nil {
		return nil, err
	}
	for componentName, componentFiles := range filesMap {
		componentFilesMap, err := doSplitFilesByComponent(componentFiles)
		if err != nil {
			return nil, err
		}
		if len(componentFilesMap) > 1 {
			return map[string]Files{}, packages.ViolationError{
				Reason:    packages.ViolationReasonNestedMultiComponentPkg,
				Component: componentName,
			}
		}
	}

	_, exists := filesMap[component]
	if !exists {
		return nil, packages.ViolationError{Reason: packages.ViolationReasonComponentNotFound, Component: component}
	}

	return filesMap, nil
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

func doSplitFilesByComponent(files Files) (map[string]Files, error) {
	paths := make([]string, 0, len(files))
	for key := range files {
		paths = append(paths, key)
	}
	sort.Strings(paths)

	filesMap := map[string]Files{}
	manifMap := map[string]bool{}

	for _, path := range paths {
		componentName, componentPath, err := getComponentNameAndPath(path)
		if err != nil {
			return map[string]Files{}, err
		}

		if _, exists := filesMap[componentName]; !exists {
			filesMap[componentName] = Files{}
			manifMap[componentName] = false
		}

		if packages.IsManifestFile(componentPath) {
			if manifestAlreadyExists, entryExists := manifMap[componentName]; entryExists && manifestAlreadyExists {
				return map[string]Files{}, packages.ViolationError{
					Reason: packages.ViolationReasonPackageManifestDuplicated,
					Path:   componentPath,
				}
			}
			manifMap[componentName] = true
		}

		filesMap[componentName][componentPath] = files[path]
	}
	return filesMap, nil
}

func isMultiComponent(manifest *manifestsv1alpha1.PackageManifest) bool {
	return manifest != nil && manifest.Spec.Components != nil
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
