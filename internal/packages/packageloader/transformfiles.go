package packageloader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/packages/packagecontent"
)

var _ FilesTransformer = (*TemplateTransformer)(nil)

// Runs a go-template transformer on all .yml or .yaml files.
type TemplateTransformer struct {
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

func NewTemplateTransformer(tmplCtx manifestsv1alpha1.TemplateContext) (*TemplateTransformer, error) {
	tctx := struct {
		Package manifestsv1alpha1.TemplateContextPackage `json:"package"`
		Config  map[string]string                        `json:"config"`
	}{tmplCtx.Package, map[string]string{}}

	switch {
	case tmplCtx.Config == nil:
	case len(tmplCtx.Config.Raw) > 0 && tmplCtx.Config.Object != nil:
		return nil, ErrDuplicateConfig
	case len(tmplCtx.Config.Raw) > 0:
		if err := json.Unmarshal(tmplCtx.Config.Raw, &tctx.Config); err != nil {
			return nil, err
		}
	}

	p, err := json.Marshal(tctx)
	if err != nil {
		return nil, err
	}

	actualCtx := map[string]interface{}{}
	if err := json.Unmarshal(p, &actualCtx); err != nil {
		return nil, err
	}

	workaroundnovalue(actualCtx)

	return &TemplateTransformer{actualCtx}, nil
}

func (t *TemplateTransformer) transform(ctx context.Context, path string, content []byte) ([]byte, error) {
	if !packages.IsTemplateFile(path) {
		// Not a template file, skip.
		return content, nil
	}

	template, err := template.New("").Option("missingkey=error").Parse(string(content))
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

func (t *TemplateTransformer) TransformPackageFiles(ctx context.Context, fileMap packagecontent.Files) error {
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
