package packageloader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/transform"
)

var _ FilesTransformer = (*PackageFileTemplateTransformer)(nil)

// Runs a go-template transformer on all .yml or .yaml files.
type PackageFileTemplateTransformer struct {
	tctx map[string]interface{}
}

func workaroundnovalue(actualCtx map[string]interface{}) {
	// The go templating engine substitutes missing values in map with "<no value>".
	// For our case we would like to raise an error in case of missing values, which can be done by setting the templater option missingkey=error...
	// except that it is ignored if the map is of type map[string]interface{} for some reason ʕ •ᴥ•ʔ. See https://github.com/golang/go/issues/24963.
	// Circumventing this by defaulting annotations and labels maps in metadata.

	metadata := actualCtx["package"].(map[string]interface{})["metadata"].(map[string]interface{})
	if metadata["annotations"] == nil {
		metadata["annotations"] = map[string]string{}
	}
	if metadata["labels"] == nil {
		metadata["labels"] = map[string]string{}
	}
}

type PackageFileTemplateContext struct {
	Package manifestsv1alpha1.TemplateContextPackage `json:"package"`
	Config  map[string]interface{}                   `json:"config"`
	Images  map[string]string                        `json:"images"`
}

func NewTemplateTransformer(tmplCtx PackageFileTemplateContext) (*PackageFileTemplateTransformer, error) {
	p, err := json.Marshal(tmplCtx)
	if err != nil {
		return nil, err
	}

	actualCtx := map[string]interface{}{}
	if err := json.Unmarshal(p, &actualCtx); err != nil {
		return nil, err
	}

	workaroundnovalue(actualCtx)

	return &PackageFileTemplateTransformer{actualCtx}, nil
}

func (t *PackageFileTemplateTransformer) transform(_ context.Context, path string, content []byte) ([]byte, error) {
	if !packages.IsTemplateFile(path) {
		// Not a template file, skip.
		return content, nil
	}

	template, err := transform.TemplateWithSprigFuncs(string(content))
	if err != nil {
		return nil, fmt.Errorf(
			"parsing template from %s: %w", path, err)
	}

	var doc bytes.Buffer
	if err := template.Execute(&doc, t.tctx); err != nil {
		return nil, fmt.Errorf(
			"executing template from %s with context %+v: %w", path, t.tctx, err)
	}
	return doc.Bytes(), nil
}

func (t *PackageFileTemplateTransformer) TransformPackageFiles(ctx context.Context, fileMap packagecontent.Files) error {
	for path, content := range fileMap {
		var err error
		content, err = t.transform(ctx, path, content)
		if err != nil {
			return err
		}
		// save back to file map without the template suffix
		fileMap[packages.StripTemplateSuffix(path)] = content
	}

	return nil
}
