package packagerender

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
)

// Renders all .yml and .yaml files into Kubernetes Objects.
func RenderObjects(
	ctx context.Context, pkg *packagetypes.Package,
	tmplCtx packagetypes.PackageRenderContext,
	validator packagetypes.ObjectValidator,
) (
	[]unstructured.Unstructured, error,
) {
	macros, err := evaluateCELMacros(pkg.Manifest.Spec.CelMacros, &tmplCtx)
	if err != nil {
		return nil, err
	}

	pathObjectMap := map[string][]unstructured.Unstructured{}

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
				pathObjectMap[path] = objects
			}
		}
	}

	if validator != nil {
		if err := validator.ValidateObjects(ctx, pkg.Manifest, pathObjectMap); err != nil {
			return nil, err
		}
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

	return filterWithCELAnnotation(objects, &tmplCtx, macros)
}

var splitYAMLDocumentsRegEx = regexp.MustCompile(`(?m)^---$`)

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
	for idx, yamlDocument := range splitYAMLDocumentsRegEx.Split(string(bytes.Trim(content, "---\n")), -1) {
		obj := unstructured.Unstructured{}
		if err = yaml.Unmarshal([]byte(yamlDocument), &obj); err != nil {
			err = packagetypes.ViolationError{
				Reason:  packagetypes.ViolationReasonInvalidYAML,
				Details: err.Error(),
				Path:    path,
				Index:   ptr.To(idx),
				Subject: yamlDocument,
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

func filterWithCELAnnotation(
	objects []unstructured.Unstructured,
	tmplCtx *packagetypes.PackageRenderContext,
	macros map[string]string,
) (
	[]unstructured.Unstructured, error,
) {
	filtered := make([]unstructured.Unstructured, 0, len(objects))

	for _, obj := range objects {
		cel, ok := obj.GetAnnotations()[v1alpha1.PackageCELConditionAnnotation]
		if !ok {
			filtered = append(filtered, obj)
			continue
		}

		cel = replaceMacros(cel, macros)

		celResult, err := evaluateCELCondition(cel, tmplCtx)
		if err != nil {
			return nil, packagetypes.ViolationError{
				Reason:  packagetypes.ViolationReasonInvalidCELExpression,
				Details: err.Error(),
				Subject: obj.GetName(),
			}
		}

		if celResult {
			filtered = append(filtered, obj)
		}
	}

	return filtered, nil
}

func replaceMacros(expression string, macros map[string]string) string {
	if macros == nil || len(macros) == 0 {
		return expression
	}

	result := expression
	for name, eval := range macros {
		result = strings.ReplaceAll(result, name, eval)
	}
	return result
}

var (
	ErrDuplicateCELMacroName = errors.New("duplicate CEL macro name")
	ErrCELMacroEvaluation    = errors.New("CEL macro evaluation failed")
	ErrInvalidCELMacroName   = errors.New("invalid CEL macro name")
	macroNameRegexp          = regexp.MustCompile("^[_a-zA-Z][_a-zA-Z0-9]*$")
)

func evaluateCELMacros(
	macros []manifests.PackageManifestCelMacro,
	tmplCtx *packagetypes.PackageRenderContext,
) (
	map[string]string, error,
) {
	if macros == nil {
		return map[string]string{}, nil
	}

	evaluation := make(map[string]string, len(macros))
	for _, m := range macros {
		// validate macro name
		if !macroNameRegexp.MatchString(m.Name) {
			return nil, fmt.Errorf("%w: '%s'", ErrInvalidCELMacroName, m.Name)
		}

		// make sure name is unique
		if _, ok := evaluation[m.Name]; ok {
			return nil, fmt.Errorf("%w: '%s'", ErrDuplicateCELMacroName, m.Name)
		}

		result, err := evaluateCELCondition(m.Expression, tmplCtx)
		if err != nil {
			return nil, fmt.Errorf("%w: '%s': %w", ErrCELMacroEvaluation, m.Name, err)
		}

		if result {
			evaluation[m.Name] = "true"
		} else {
			evaluation[m.Name] = "false"
		}
	}

	return evaluation, nil
}
