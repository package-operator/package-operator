package packagebytes

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
