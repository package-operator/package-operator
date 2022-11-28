package packagebytes

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var errExample = errors.New("example error")

func TestTransformerList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t1 := &TransformerMock{}
		t2 := &TransformerMock{}

		t1.On("Transform", mock.Anything, mock.Anything).Return(nil)
		t2.On("Transform", mock.Anything, mock.Anything).Return(nil)

		tl := TransformerList{
			t1, t2,
		}

		ctx := context.Background()
		err := tl.Transform(ctx, FileMap{})
		require.NoError(t, err)

		t1.AssertCalled(t, "Transform", mock.Anything, mock.Anything)
		t2.AssertCalled(t, "Transform", mock.Anything, mock.Anything)
	})

	t.Run("stops at first error", func(t *testing.T) {
		t1 := &TransformerMock{}
		t2 := &TransformerMock{}

		t1.On("Transform", mock.Anything, mock.Anything).
			Return(errExample)
		t2.On("Transform", mock.Anything, mock.Anything).Return(nil)

		tl := TransformerList{
			t1, t2,
		}

		ctx := context.Background()
		err := tl.Transform(ctx, FileMap{})
		assert.EqualError(t, err, errExample.Error())

		t1.AssertCalled(t, "Transform", mock.Anything, mock.Anything)
		t2.AssertNotCalled(t, "Transform", mock.Anything, mock.Anything)
	})
}

func TestTemplateTransformer(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tt := &TemplateTransformer{
			TemplateContext: TemplateContext{
				Package: PackageTemplateContext{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
		}

		template := []byte("#{{.Package.Name}}#")
		fm := FileMap{
			"something": template,
			"test.yaml": template,
			"test.yml":  template,
		}

		ctx := context.Background()
		err := tt.Transform(ctx, fm)
		require.NoError(t, err)

		templateResult := "#test#"
		assert.Equal(t, templateResult, string(fm["test.yaml"]))
		assert.Equal(t, templateResult, string(fm["test.yml"]))
		// only touches YAML files
		assert.Equal(t, string(template), string(fm["something"]))
	})

	t.Run("invalid template", func(t *testing.T) {
		tt := &TemplateTransformer{
			TemplateContext: TemplateContext{
				Package: PackageTemplateContext{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
		}

		template := []byte("#{{.Package.Name}#")
		fm := FileMap{
			"test.yaml": template,
		}

		ctx := context.Background()
		err := tt.Transform(ctx, fm)
		require.Error(t, err)
	})

	t.Run("execution template error", func(t *testing.T) {
		tt := &TemplateTransformer{
			TemplateContext: TemplateContext{
				Package: PackageTemplateContext{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
		}

		template := []byte("#{{.Package.Banana}}#")
		fm := FileMap{
			"test.yaml": template,
		}

		ctx := context.Background()
		err := tt.Transform(ctx, fm)
		require.Error(t, err)
	})
}

var (
	_ Transformer = (*TransformerMock)(nil)
)

type TransformerMock struct {
	mock.Mock
}

func (m *TransformerMock) Transform(ctx context.Context, fileMap FileMap) error {
	args := m.Called(ctx, fileMap)
	return args.Error(0)
}
