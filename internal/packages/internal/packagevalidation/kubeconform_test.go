package packagevalidation

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/yannh/kubeconform/pkg/validator"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
)

var errTest = errors.New("test")

func Test_runKubeconformForFile(t *testing.T) {
	t.Parallel()

	kvm := &kubeconformValidatorMock{}
	kvm.
		On("Validate", mock.Anything, mock.Anything, mock.Anything).
		Return([]validator.Result{
			{
				Status: validator.Invalid,
				Err:    errTest,
			},
		})

	verrs, err := runKubeconformForFile("xxx.yaml", nil, kvm)
	require.NoError(t, err)
	if assert.Len(t, verrs, 1) {
		var verr packagetypes.ViolationError
		require.ErrorAs(t, verrs[0], &verr)

		assert.Equal(t, "test", verr.Details)
		assert.Equal(t, "xxx.yaml", verr.Path)
		assert.Equal(t, packagetypes.ViolationReasonKubeconform, verr.Reason)
	}
}

func Test_defaultKubeconformSchemaLocations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  []string
		output []string
	}{
		{
			name: "default if empty",
			output: []string{
				defaultKubeSchemaLocation,
				defaultCRDSSchemaLocation,
			},
		},
		{
			name: "passthrough if set",
			input: []string{
				"stuff",
			},
			output: []string{
				"stuff",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			out := defaultKubeconformSchemaLocations(&manifests.PackageManifest{
				Test: manifests.PackageManifestTest{
					Kubeconform: &manifests.PackageManifestTestKubeconform{
						SchemaLocations: test.input,
					},
				},
			})
			assert.Equal(t, test.output, out)
		})
	}
}

type kubeconformValidatorMock struct {
	mock.Mock
}

func (m *kubeconformValidatorMock) Validate(
	path string, file io.ReadCloser,
) []validator.Result {
	args := m.Called(path, file)
	return args.Get(0).([]validator.Result)
}
