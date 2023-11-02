package objecttemplate

import (
	"bytes"
	"context"
	"encoding/json"

	"package-operator.run/internal/transform"
)

type TemplateContext struct {
	Config      map[string]any `json:"config"`
	Environment map[string]any `json:"environment"`
}

type TemplateTransformer struct {
	tctx map[string]any
}

func NewTemplateTransformer(tmplCtx TemplateContext) (*TemplateTransformer, error) {
	p, err := json.Marshal(tmplCtx)
	if err != nil {
		return nil, err
	}

	actualCtx := map[string]any{}
	if err := json.Unmarshal(p, &actualCtx); err != nil {
		return nil, err
	}

	return &TemplateTransformer{actualCtx}, nil
}

func (t *TemplateTransformer) transform(_ context.Context, content []byte) ([]byte, error) {
	template, err := transform.TemplateWithSprigFuncs(string(content))
	if err != nil {
		return nil, &TemplateError{Err: err}
	}

	var doc bytes.Buffer
	if err := template.Execute(&doc, t.tctx); err != nil {
		return nil, &TemplateError{Err: err}
	}
	return doc.Bytes(), nil
}
