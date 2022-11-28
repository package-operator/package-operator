package packagebytes

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"package-operator.run/package-operator/internal/packages"
)

type Transformer interface {
	Transform(ctx context.Context, fileMap FileMap) error
}

var (
	_ Transformer = (TransformerList)(nil)
	_ Transformer = (*TemplateTransformer)(nil)
)

// Applies a list of BytesTransformer to the given content.
type TransformerList []Transformer

func (l TransformerList) Transform(ctx context.Context, fileMap FileMap) error {
	for _, t := range l {
		if err := t.Transform(ctx, fileMap); err != nil {
			return err
		}
	}
	return nil
}

// Template Context provided when executing file templates.
type TemplateContext struct {
	Package PackageTemplateContext
}

type PackageTemplateContext struct {
	metav1.ObjectMeta
}

// Runs a go-template transformer on all .yml or .yaml files.
type TemplateTransformer struct {
	TemplateContext TemplateContext
}

func (t *TemplateTransformer) Transform(ctx context.Context, fileMap FileMap) error {
	for path, content := range fileMap {
		var err error
		content, err = t.transform(ctx, path, content)
		if err != nil {
			return err
		}
		fileMap[path] = content
	}

	return nil
}

func (t *TemplateTransformer) transform(ctx context.Context, path string, content []byte) ([]byte, error) {
	if !packages.IsYAMLFile(path) || packages.IsManifestFile(path) {
		// It is the package manifest file or not a YAML file, skipping
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
