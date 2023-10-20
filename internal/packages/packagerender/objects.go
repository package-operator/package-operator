package packagerender

import (
	"bytes"
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/packagetypes"
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
	var objects []unstructured.Unstructured
	for _, objs := range pathObjectMap {
		objects = append(objects, objs...)
	}
	return objects, nil
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
