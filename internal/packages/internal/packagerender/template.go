package packagerender

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"text/template"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagerender/celctx"

	"package-operator.run/internal/packages/internal/packagetypes"
	"package-operator.run/internal/transform"
)

var errConstructingCelContext = errors.New("constructing CEL context")

// Runs a go-template transformer on all .gotmpl files.
func RenderTemplates(_ context.Context, pkg *packagetypes.Package, tmplCtx packagetypes.PackageRenderContext) error {
	tctx, err := templateContext(tmplCtx)
	if err != nil {
		return err
	}

	templ := template.New("pkg").Option("missingkey=error")
	templ = templ.Funcs(transform.SprigFuncs(templ)).
		Funcs(transform.FileFuncs(pkg.Files))

	celFn, err := celTemplateFunction(pkg.Manifest.Spec.Filters.Conditions, tmplCtx)
	if err != nil {
		return fmt.Errorf("%w: %w", errConstructingCelContext, err)
	}
	templ = templ.Funcs(celFn)

	// gather all templates to allow cross-file declarations and reuse of helpers.
	for path, content := range pkg.Files {
		if !packagetypes.IsTemplateFile(path) {
			// Not a template file, skip.
			continue
		}

		_, err := templ.New(path).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parsing template from %s: %w", path, err)
		}
	}

	for path := range pkg.Files {
		if !packagetypes.IsTemplateFile(path) {
			// Not a template file, skip.
			continue
		}

		var buf bytes.Buffer
		if err := templ.ExecuteTemplate(&buf, path, tctx); err != nil {
			return fmt.Errorf("executing template from %s with context %+v: %w", path, tctx, err)
		}

		// save back to file map without the template suffix
		pkg.Files[packagetypes.StripTemplateSuffix(path)] = buf.Bytes()
	}

	return nil
}

func templateContext(tmplCtx packagetypes.PackageRenderContext) (map[string]any, error) {
	p, err := json.Marshal(tmplCtx)
	if err != nil {
		return nil, err
	}

	actualCtx := map[string]any{}
	if err := json.Unmarshal(p, &actualCtx); err != nil {
		return nil, err
	}
	workaroundnovalue(actualCtx)
	return actualCtx, nil
}

func workaroundnovalue(actualCtx map[string]any) {
	// The go templating engine substitutes missing values in map with "<no value>".
	// For our case we would like to raise an error in case of missing values, which
	// can be done by setting the templater option missingkey=error... except that
	// it is ignored if the map is of type map[string]any for some reason
	// ʕ •ᴥ•ʔ. See https://github.com/golang/go/issues/24963.
	// Circumventing this by defaulting annotations and labels maps in metadata.

	metadata := actualCtx["package"].(map[string]any)["metadata"].(map[string]any)
	if metadata["annotations"] == nil {
		metadata["annotations"] = map[string]string{}
	}
	if metadata["labels"] == nil {
		metadata["labels"] = map[string]string{}
	}
}

func celTemplateFunction(
	conditions []manifests.PackageManifestNamedCondition,
	tmplCtx packagetypes.PackageRenderContext,
) (
	template.FuncMap, error,
) {
	cc, err := celctx.New(conditions, tmplCtx)
	if err != nil {
		return nil, err
	}

	return template.FuncMap{
		"cel": func(expression string) (bool, error) {
			return cc.Evaluate(expression)
		},
	}, nil
}
