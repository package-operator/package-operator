package probing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ Prober = (*proberMock)(nil)

type proberMock struct {
	mock.Mock
}

func (m *proberMock) Probe(obj *unstructured.Unstructured) (
	success bool, message string,
) {
	args := m.Called(obj)
	return args.Bool(0), args.String(1)
}

func TestAnd(t *testing.T) {
	t.Parallel()
	t.Run("combines failure messages", func(t *testing.T) {
		t.Parallel()
		prober1 := &proberMock{}
		prober2 := &proberMock{}

		prober1.
			On("Probe", mock.Anything).
			Return(false, "error from prober1")
		prober2.
			On("Probe", mock.Anything).
			Return(false, "error from prober2")

		l := And{prober1, prober2}

		s, m := l.Probe(&unstructured.Unstructured{})
		assert.False(t, s)
		assert.Equal(t, "error from prober1, error from prober2", m)
	})
	t.Run("succeeds when all subprobes succeed", func(t *testing.T) {
		t.Parallel()
		prober1 := &proberMock{}
		prober2 := &proberMock{}

		prober1.
			On("Probe", mock.Anything).
			Return(true, "")
		prober2.
			On("Probe", mock.Anything).
			Return(true, "")

		l := And{prober1, prober2}

		s, m := l.Probe(&unstructured.Unstructured{})
		assert.True(t, s)
		assert.Equal(t, "", m)
	})
}
