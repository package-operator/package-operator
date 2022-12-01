package packagebytes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestValidatorList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t1 := &ValidatorMock{}
		t2 := &ValidatorMock{}

		t1.On("Validate", mock.Anything, mock.Anything).Return(nil)
		t2.On("Validate", mock.Anything, mock.Anything).Return(nil)

		tl := ValidatorList{
			t1, t2,
		}

		ctx := context.Background()
		err := tl.Validate(ctx, FileMap{})
		require.Nil(t, err)

		t1.AssertCalled(t, "Validate", mock.Anything, mock.Anything)
		t2.AssertCalled(t, "Validate", mock.Anything, mock.Anything)
	})
}

var (
	_ Validator = (*ValidatorMock)(nil)
)

type ValidatorMock struct {
	mock.Mock
}

func (m *ValidatorMock) Validate(ctx context.Context, fileMap FileMap) error {
	args := m.Called(ctx, fileMap)
	return args.Error(0)
}
