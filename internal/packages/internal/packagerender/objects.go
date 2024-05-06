package packagerender

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagerender/celctx"
	"package-operator.run/internal/packages/internal/packagetypes"
)

// Renders all .yml and .yaml files into Kubernetes Objects.
func RenderObjects(
	ctx context.Context, pkg *packagetypes.Package,
	tmplCtx packagetypes.PackageRenderContext,
	validator packagetypes.ObjectValidator,
) (
	pathObject map[string][]unstructured.Unstructured,
	err error,
) {
	pathObject = map[string][]unstructured.Unstructured{}
	for path, content := range pkg.Files {
		switch {
		case strings.HasPrefix(filepath.Base(path), "_"):
			// skip template helper files.
		case !packagetypes.IsYAMLFile(path):
			// skip non YAML files
		default:
			objects, err := parseObjects(pkg.Manifest, tmplCtx, path, content)
			if err != nil {
				return nil, err
			}
			if len(objects) != 0 {
				pathObject[path] = objects
			}
		}
	}

	if validator != nil {
		if err := validator.ValidateObjects(ctx, pkg.Manifest, pathObject); err != nil {
			return nil, err
		}
	}
	return pathObject, nil
}

// Renders all .yml and .yaml files into Kubernetes Objects and applies CEL conditionals to filter objects.
// Will return a map[path]index of filtered object.
func RenderObjectsWithFilterInfo(
	ctx context.Context, pkg *packagetypes.Package,
	tmplCtx packagetypes.PackageRenderContext,
	validator packagetypes.ObjectValidator,
) (
	pathObject map[string][]unstructured.Unstructured,
	pathFilteredIndex map[string][]int,
	err error,
) {
	pathObject, err = RenderObjects(ctx, pkg, tmplCtx, validator)
	if err != nil {
		return nil, nil, err
	}

	pathFilteredIndex, err = filterWithCEL(pathObject, pkg.Manifest.Spec.ConditionalFiltering, tmplCtx)
	if err != nil {
		return nil, nil, err
	}
	return pathObject, pathFilteredIndex, nil
}

// Renders all .yml and .yaml files into Kubernetes Objects and applies CEL conditionals to filter objects.
func RenderObjectsWithFilter(
	ctx context.Context, pkg *packagetypes.Package,
	tmplCtx packagetypes.PackageRenderContext,
	validator packagetypes.ObjectValidator,
) (
	[]unstructured.Unstructured, error,
) {
	pathObjectMap, _, err := RenderObjectsWithFilterInfo(
		ctx, pkg, tmplCtx, validator,
	)
	if err != nil {
		return nil, err
	}

	// sort paths to have a deterministic output.
	paths := make([]string, len(pathObjectMap))
	var i int
	for path := range pathObjectMap {
		paths[i] = path
		i++
	}
	// sorts a list of file paths ascending.
	// e.g. a, a/b, a/b/c, b, b/x, bat.
	sort.Slice(paths, func(i, j int) bool {
		p1 := strings.ReplaceAll(paths[i], "/", "\x00")
		p2 := strings.ReplaceAll(paths[j], "/", "\x00")
		return p1 < p2
	})

	var objects []unstructured.Unstructured
	for _, path := range paths {
		objs := pathObjectMap[path]
		objects = append(objects, objs...)
	}

	return objects, nil
}

func parseObjects(
	manifest *manifests.PackageManifest,
	tmplCtx packagetypes.PackageRenderContext,
	path string, content []byte,
) (
	objects []unstructured.Unstructured,
	err error,
) {
	objects = []unstructured.Unstructured{}

	// Split for every included yaml document.
	for idx, yamlDocument := range packagetypes.SplitYAMLDocuments(content) {
		obj := unstructured.Unstructured{}
		if err = yaml.Unmarshal(yamlDocument, &obj); err != nil {
			err = packagetypes.ViolationError{
				Reason:  packagetypes.ViolationReasonInvalidYAML,
				Details: err.Error(),
				Path:    path,
				Index:   ptr.To(idx),
				Subject: string(yamlDocument),
			}
			return
		}

		if len(obj.Object) != 0 {
			obj.SetLabels(labels.Merge(obj.GetLabels(), commonLabels(manifest, tmplCtx.Package.Name)))
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

func commonLabels(manifest *manifests.PackageManifest, packageName string) map[string]string {
	return map[string]string{
		manifests.PackageLabel:         manifest.Name,
		manifests.PackageInstanceLabel: packageName,
	}
}

func filterWithCEL(
	pathObjectMap map[string][]unstructured.Unstructured,
	condFiltering manifests.PackageManifestConditionalFiltering,
	tmplCtx packagetypes.PackageRenderContext,
) (pathFilteredIndex map[string][]int, err error) {
	// Create CEL evaluation environment
	cc, err := celctx.New(condFiltering.NamedConditions, tmplCtx)
	if err != nil {
		return nil, err
	}

	pathsToExclude, err := computeIgnoredPaths(condFiltering.ConditionalPaths, cc)
	if err != nil {
		return nil, err
	}

	pathFilteredIndex = map[string][]int{}
	for path, objects := range pathObjectMap {
		exclude, err := isExcluded(path, pathsToExclude)
		if err != nil {
			return nil, err
		}
		if exclude {
			delete(pathObjectMap, path)
			continue
		}

		var filteredIndex []int
		pathObjectMap[path], filteredIndex, err = filterWithCELAnnotation(objects, cc)
		if err != nil {
			return nil, err
		}
		if filteredIndex != nil {
			pathFilteredIndex[path] = filteredIndex
		}
	}

	return pathFilteredIndex, nil
}

// Filters a list of object on their "package-operator.run/condition" annotation,
// by evaluating the contained CEL expression and returning a list of filtered objects
// and the indexes that got removed.
func filterWithCELAnnotation(
	objects []unstructured.Unstructured,
	cc *celctx.CelCtx,
) (
	filtered []unstructured.Unstructured,
	filteredIndexes []int,
	err error,
) {
	for i, obj := range objects {
		cel, ok := obj.GetAnnotations()[v1alpha1.PackageCELConditionAnnotation]
		// If object doesn't have CEL annotation, append it
		if !ok {
			filtered = append(filtered, obj)
			continue
		}

		celResult, err := cc.Evaluate(cel)
		if err != nil {
			return nil, nil, packagetypes.ViolationError{
				Reason:  packagetypes.ViolationReasonInvalidCELExpression,
				Details: err.Error(),
				Subject: obj.GetName(),
			}
		}

		// If CEL annotation evaluates to true, append object
		if celResult {
			filtered = append(filtered, obj)
		} else {
			filteredIndexes = append(filteredIndexes, i)
		}
	}

	return filtered, filteredIndexes, nil
}

var ErrInvalidConditionalPathsExpression = errors.New("invalid spec.conditionalPaths expression")

func computeIgnoredPaths(
	conditionalPaths []manifests.PackageManifestConditionalPath,
	cc *celctx.CelCtx,
) (
	[]string, error,
) {
	var globs []string
	for _, cp := range conditionalPaths {
		result, err := cc.Evaluate(cp.Expression)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrInvalidConditionalPathsExpression, cp.Glob, err)
		}

		// If expression evaluates to false, add glob to ignored paths
		if !result {
			globs = append(globs, cp.Glob)
		}
	}

	return globs, nil
}

func isExcluded(path string, pathsToExclude []string) (exclude bool, err error) {
	for _, glob := range pathsToExclude {
		exclude, err = doublestar.PathMatch(glob, path)
		if err != nil || exclude {
			return exclude, err
		}
	}
	return false, nil
}
