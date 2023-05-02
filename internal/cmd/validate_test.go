package cmd

import (
	"context"
	_ "embed"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"

	"package-operator.run/package-operator/internal/packages/packagecontent"
)

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	testLogger := testr.New(t)

	for name, tc := range map[string]struct {
		Options  []ValidateOption
		Expected ValidateConfig
	}{
		"defaults": {
			Expected: ValidateConfig{
				Log:    logr.Discard(),
				Puller: &defaultPackagePuller{},
			},
		},
		"with logger": {
			Options: []ValidateOption{
				WithLog{Log: testLogger},
			},
			Expected: ValidateConfig{
				Log:    testLogger,
				Puller: &defaultPackagePuller{},
			},
		},
	} {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var cfg ValidateConfig

			cfg.Option(tc.Options...)
			cfg.Default()

			assert.Equal(t, tc.Expected, cfg)
		})
	}
}

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
		tc := tc

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
		Options     []ValidatePackageOption
		PulledFiles packagecontent.Files
		Assertion   require.ErrorAssertionFunc
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
			PulledFiles: packagecontent.Files{
				"manifest.yaml": _manifest,
			},
			Assertion: require.NoError,
		},
		"invalid remote reference": {
			Options: []ValidatePackageOption{
				WithRemoteReference("test"),
			},
			PulledFiles: packagecontent.Files{
				"garbage.trash": []byte{12, 34, 56, 78},
			},
			Assertion: require.Error,
		},
	} {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mPuller := &packagePullerMock{}

			if len(tc.PulledFiles) > 0 {
				mPuller.
					On("PullPackage", mock.Anything, "test").
					Return(tc.PulledFiles, nil)
			}

			validate := NewValidate(
				WithPackagePuller{Puller: mPuller},
			)

			tc.Assertion(t, validate.ValidatePackage(context.Background(), tc.Options...))
		})
	}
}

type packagePullerMock struct {
	mock.Mock
}

func (m *packagePullerMock) PullPackage(ctx context.Context, ref string) (packagecontent.Files, error) {
	args := m.Called(ctx, ref)

	return args.Get(0).(packagecontent.Files), args.Error(1)
}
