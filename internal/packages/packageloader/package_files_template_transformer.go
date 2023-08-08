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
	Package     manifestsv1alpha1.TemplateContextPackage `json:"package"`
	Config      map[string]interface{}                   `json:"config"`
	Images      map[string]string                        `json:"images"`
	Environment manifestsv1alpha1.PackageEnvironment     `json:"environment"`
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

func (t *PackageFileTemplateTransformer) TransformPackageFiles(_ context.Context, fileMap packagecontent.Files) error {
	templ := template.New("pkg").Option("missingkey=error").Funcs(transform.SprigFuncs())

	// gather all templates to allow cross-file declarations and reuse of helpers.
	for path, content := range fileMap {
		if !packages.IsTemplateFile(path) {
			// Not a template file, skip.
			continue
		}

		_, err := templ.New(path).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parsing template from %s: %w", path, err)
		}
	}

	for path := range fileMap {
		if !packages.IsTemplateFile(path) {
			// Not a template file, skip.
			continue
		}

		var buf bytes.Buffer
		if err := templ.ExecuteTemplate(&buf, path, t.tctx); err != nil {
			return fmt.Errorf("executing template from %s with context %+v: %w", path, t.tctx, err)
		}

		// save back to file map without the template suffix
		fileMap[packages.StripTemplateSuffix(path)] = buf.Bytes()
	}

	return nil
}
