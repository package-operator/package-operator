package cmd

import (
	"context"
	_ "embed"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages"
)

func TestValidatePackageConfig(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		Options   []ValidatePackageOption
		Assertion require.ErrorAssertionFunc
	}{
		"no options": {
			Options:   []ValidatePackageOption{},
			Assertion: require.Error,
		},
		"empty path": {
			Options: []ValidatePackageOption{
				WithPath(""),
			},
			Assertion: require.Error,
		},
		"empty remote reference": {
			Options: []ValidatePackageOption{
				WithRemoteReference(""),
			},
			Assertion: require.Error,
		},
		"mutually exclusive options": {
			Options: []ValidatePackageOption{
				WithPath("test"),
				WithRemoteReference("test"),
			},
			Assertion: require.Error,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var cfg ValidatePackageConfig

			cfg.Option(tc.Options...)

			tc.Assertion(t, cfg.Validate())
		})
	}
}

//go:embed testdata/manifest.yaml
var _manifest []byte

func TestValidate_ValidatePackage(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		Options   []ValidatePackageOption
		PulledPkg *packages.RawPackage
		Assertion require.ErrorAssertionFunc
	}{
		"valid path": {
			Options: []ValidatePackageOption{
				WithPath("testdata"),
			},
			Assertion: require.NoError,
		},
		"invalid path": {
			Options: []ValidatePackageOption{
				WithPath("dne"),
			},
			Assertion: require.Error,
		},
		"remote reference": {
			Options: []ValidatePackageOption{
				WithRemoteReference("test"),
			},
			PulledPkg: &packages.RawPackage{
				Files: packages.Files{
					"manifest.yaml": _manifest,
				},
			},
			Assertion: require.NoError,
		},
		"invalid remote reference": {
			Options: []ValidatePackageOption{
				WithRemoteReference("test"),
			},
			PulledPkg: &packages.RawPackage{
				Files: packages.Files{
					"garbage.trash": []byte{12, 34, 56, 78},
				},
			},
			Assertion: require.Error,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scheme, err := NewScheme()
			require.NoError(t, err)

			mPuller := &pullerMock{}

			if tc.PulledPkg != nil {
				mPuller.
					On("Pull", mock.Anything, "test", mock.Anything).
					Return(tc.PulledPkg, nil)
			}

			validate := NewValidate(
				scheme,
				WithPuller{Pull: mPuller.Pull},
			)

			tc.Assertion(t, validate.ValidatePackage(context.Background(), tc.Options...))
		})
	}
}

type pullerMock struct {
	mock.Mock
}

func (m *pullerMock) Pull(ctx context.Context, ref string, opts ...crane.Option) (*packages.RawPackage, error) {
	actualArgs := make([]any, 0, 2+len(opts))
	actualArgs = append(actualArgs, ctx, ref)
	for _, opt := range opts {
		actualArgs = append(actualArgs, opt)
	}

	args := m.Called(actualArgs...)
	rawPkg, _ := args.Get(0).(*packages.RawPackage)
	return rawPkg, args.Error(1)
}
