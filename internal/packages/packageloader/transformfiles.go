package packageloader

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/packages/packagecontent"
)

var _ FilesTransformer = (*TemplateTransformer)(nil)

// Runs a go-template transformer on all .yml or .yaml files.
type TemplateTransformer struct {
	TemplateContext manifestsv1alpha1.TemplateContext
}

func (t *TemplateTransformer) transform(ctx context.Context, path string, content []byte) ([]byte, error) {
	if !packages.IsTemplateFile(path) {
		// Not a template file, skip.
		return content, nil
	}

	template, err := template.New("").Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf(
			"parsing template from %s: %w", path, err)
	}

	var doc bytes.Buffer
	if err := template.Execute(&doc, t.TemplateContext); err != nil {
		return nil, fmt.Errorf(
			"executing template from %s: %w", path, err)
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
