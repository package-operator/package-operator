package treecmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	internalcmd "package-operator.run/internal/cmd"
)

func TestTree_Success(t *testing.T) {
	t.Parallel()

	t.Run("namespace scoped", func(t *testing.T) {
		t.Parallel()

		scheme, err := internalcmd.NewScheme()
		require.NoError(t, err)

		factory := &rendererFactoryMock{}
		factory.On("Renderer").Return(internalcmd.NewTree(scheme))

		cmd := NewCmd(factory)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"--config-testcase", "namespace-scope", "testdata"})

		require.NoError(t, cmd.Execute())
		require.Len(t, stderr.String(), 0)

		const expectedOutput = `test-stub
Package namespace/name
└── Phase deploy
    └── apps/v1, Kind=Deployment /test-stub-name
    └── apps/v1, Kind=Deployment external-name/test-external-name (EXTERNAL)
`
		assert.Equal(t, expectedOutput, stdout.String())
	})

	t.Run("cluster scoped", func(t *testing.T) {
		t.Parallel()

		scheme, err := internalcmd.NewScheme()
		require.NoError(t, err)

		factory := &rendererFactoryMock{}
		factory.On("Renderer").Return(internalcmd.NewTree(scheme))

		cmd := NewCmd(factory)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"--config-testcase", "namespace-scope", "--cluster", "testdata"})

		require.NoError(t, cmd.Execute())
		require.Len(t, stderr.String(), 0)

		const expectedOutput = `test-stub
ClusterPackage /name
└── Phase namespace
│   ├── /v1, Kind=Namespace /name
└── Phase deploy
    └── apps/v1, Kind=Deployment name/test-stub-name
    └── apps/v1, Kind=Deployment external-name/test-external-name (EXTERNAL)
`
		assert.Equal(t, expectedOutput, stdout.String())
	})
}

func TestTree_InvalidArgs(t *testing.T) {
	t.Parallel()

	t.Run("no args", func(t *testing.T) {
		t.Parallel()

		scheme, err := internalcmd.NewScheme()
		require.NoError(t, err)

		factory := &rendererFactoryMock{}
		factory.On("Renderer").Return(internalcmd.NewTree(scheme))

		cmd := NewCmd(factory)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)

		require.Error(t, cmd.Execute())
	})

	t.Run("empty source path", func(t *testing.T) {
		t.Parallel()

		scheme, err := internalcmd.NewScheme()
		require.NoError(t, err)

		factory := &rendererFactoryMock{}
		factory.On("Renderer").Return(internalcmd.NewTree(scheme))

		cmd := NewCmd(factory)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{""})

		require.Error(t, cmd.Execute())
	})

	t.Run("multi template config", func(t *testing.T) {
		t.Parallel()

		scheme, err := internalcmd.NewScheme()
		require.NoError(t, err)

		factory := &rendererFactoryMock{}
		factory.On("Renderer").Return(internalcmd.NewTree(scheme))

		cmd := NewCmd(factory)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"--config-path", "testdata/.config.yaml", "--config-testcase", "namespace-scope", "testdata"})

		require.Error(t, cmd.Execute())
	})

	t.Run("missing source", func(t *testing.T) {
		t.Parallel()

		scheme, err := internalcmd.NewScheme()
		require.NoError(t, err)

		factory := &rendererFactoryMock{}
		factory.On("Renderer").Return(internalcmd.NewTree(scheme))

		cmd := NewCmd(factory)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"invisible_chicken"})

		require.Error(t, cmd.Execute())
	})
}

func TestTree_ConfigPath(t *testing.T) {
	t.Parallel()

	t.Run("missing config path", func(t *testing.T) {
		t.Parallel()

		scheme, err := internalcmd.NewScheme()
		require.NoError(t, err)

		factory := &rendererFactoryMock{}
		factory.On("Renderer").Return(internalcmd.NewTree(scheme))

		cmd := NewCmd(factory)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"--config-path", "nonexistent", "testdata"})

		require.Error(t, cmd.Execute())
	})
}

func TestTree_ConfigTemplate(t *testing.T) {
	t.Parallel()

	t.Run("missing config path", func(t *testing.T) {
		t.Parallel()

		scheme, err := internalcmd.NewScheme()
		require.NoError(t, err)

		factory := &rendererFactoryMock{}
		factory.On("Renderer").Return(internalcmd.NewTree(scheme))

		cmd := NewCmd(factory)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"--config-testcase", "nonexistent", "testdata"})

		require.Error(t, cmd.Execute())
	})
}

func TestTree_NoConfig(t *testing.T) {
	t.Parallel()

	t.Run("namespace scoped", func(t *testing.T) {
		t.Parallel()

		scheme, err := internalcmd.NewScheme()
		require.NoError(t, err)

		factory := &rendererFactoryMock{}
		factory.On("Renderer").Return(internalcmd.NewTree(scheme))

		cmd := NewCmd(factory)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"testdata"})

		require.NoError(t, cmd.Execute())
	})

	t.Run("cluster scoped", func(t *testing.T) {
		t.Parallel()

		scheme, err := internalcmd.NewScheme()
		require.NoError(t, err)

		factory := &rendererFactoryMock{}
		factory.On("Renderer").Return(internalcmd.NewTree(scheme))

		cmd := NewCmd(factory)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"--cluster", "testdata"})

		require.NoError(t, cmd.Execute())
	})
}

func TestTree_FileConfig(t *testing.T) {
	t.Parallel()

	t.Run("namespace scoped", func(t *testing.T) {
		t.Parallel()

		scheme, err := internalcmd.NewScheme()
		require.NoError(t, err)

		factory := &rendererFactoryMock{}
		factory.On("Renderer").Return(internalcmd.NewTree(scheme))

		cmd := NewCmd(factory)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"--config-path", "testdata/.config.yaml", "testdata"})

		require.NoError(t, cmd.Execute())
		require.Len(t, stderr.String(), 0)

		const expectedOutput = `test-stub
Package namespace/name
└── Phase deploy
    └── apps/v1, Kind=Deployment /test-stub-name
    └── apps/v1, Kind=Deployment external-name/test-external-name (EXTERNAL)
`
		assert.Equal(t, expectedOutput, stdout.String())
	})

	t.Run("cluster scoped", func(t *testing.T) {
		t.Parallel()

		scheme, err := internalcmd.NewScheme()
		require.NoError(t, err)

		factory := &rendererFactoryMock{}
		factory.On("Renderer").Return(internalcmd.NewTree(scheme))

		cmd := NewCmd(factory)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"--config-path", "testdata/.config.yaml", "--cluster", "testdata"})

		require.NoError(t, cmd.Execute())
		require.Len(t, stderr.String(), 0)

		const expectedOutput = `test-stub
ClusterPackage /name
└── Phase namespace
│   ├── /v1, Kind=Namespace /name
└── Phase deploy
    └── apps/v1, Kind=Deployment name/test-stub-name
    └── apps/v1, Kind=Deployment external-name/test-external-name (EXTERNAL)
`
		assert.Equal(t, expectedOutput, stdout.String())
	})
}

type rendererFactoryMock struct {
	mock.Mock
}

func (m *rendererFactoryMock) Renderer() Renderer {
	args := m.Called()

	return args.Get(0).(Renderer)
}
