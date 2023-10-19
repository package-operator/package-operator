package packageloader

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/yannh/kubeconform/pkg/validator"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages"
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
		var verr packages.ViolationError
		require.ErrorAs(t, verrs[0], &verr)

		assert.Equal(t, "test", verr.Details)
		assert.Equal(t, "xxx.yaml", verr.Path)
		assert.Equal(t, packages.ViolationReasonKubeconform, verr.Reason)
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
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			out := defaultKubeconformSchemaLocations(&manifestsv1alpha1.PackageManifest{
				Test: manifestsv1alpha1.PackageManifestTest{
					Kubeconform: &manifestsv1alpha1.PackageManifestTestKubeconform{
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
