package probing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"pkg.package-operator.run/boxcutter/machinery/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ Prober = (*proberMock)(nil)

type proberMock struct {
	mock.Mock
}

func (m *proberMock) Probe(obj client.Object) types.ProbeResult {
	args := m.Called(obj)
	return args.Get(0).(types.ProbeResult)
}

func TestAnd(t *testing.T) {
	t.Parallel()
	t.Run("combines failure messages", func(t *testing.T) {
		t.Parallel()
		prober1 := &proberMock{}
		prober2 := &proberMock{}

		prober1.
			On("Probe", mock.Anything).
			Return(false, []string{"error from prober1"})
		prober2.
			On("Probe", mock.Anything).
			Return(false, []string{"error from prober2"})

		l := And{prober1, prober2}

		result := l.Probe(&unstructured.Unstructured{})
		assert.Equal(t, false, result.Status) //nolint: testifylint
		assert.Equal(t, []string{"error from prober1", "error from prober2"}, result.Messages)
	})
	t.Run("succeeds when all subprobes succeed", func(t *testing.T) {
		t.Parallel()
		prober1 := &proberMock{}
		prober2 := &proberMock{}

		prober1.
			On("Probe", mock.Anything).
			Return(true, []string{})
		prober2.
			On("Probe", mock.Anything).
			Return(true, []string{})

		l := And{prober1, prober2}

		result := l.Probe(&unstructured.Unstructured{})
		assert.Equal(t, true, result.Status) //nolint: testifylint
		assert.Nil(t, result.Messages)
	})
}
