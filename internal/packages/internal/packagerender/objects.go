package packagerender

import (
	"bytes"
	"context"
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
	"package-operator.run/internal/packages/internal/packagerender/celctx"
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

	return filterWithCELAnnotation(objects, pkg.Manifest.Spec.ConditionalFiltering.NamedConditions, &tmplCtx)
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
	snippets []manifests.PackageManifestNamedCondition,
	tmplCtx *packagetypes.PackageRenderContext,
) (
	[]unstructured.Unstructured, error,
) {
	cc, err := celctx.New(snippets, tmplCtx)
	if err != nil {
		return nil, err
	}

	filtered := make([]unstructured.Unstructured, 0, len(objects))

	for _, obj := range objects {
		cel, ok := obj.GetAnnotations()[v1alpha1.PackageCELConditionAnnotation]
		if !ok {
			filtered = append(filtered, obj)
			continue
		}

		celResult, err := cc.Evaluate(cel)
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
