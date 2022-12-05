package packagebytes

import (
	"context"
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
