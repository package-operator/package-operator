package treecmd_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/package-operator/cmd/kubectl-package/command"
)

func TestTree_Success(t *testing.T) {
	t.Parallel()

	t.Run("namespace scoped", func(t *testing.T) {
		t.Parallel()

		cmd := command.CobraRoot()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tree", "--config-testcase", "namespace-scope", "testdata"})

		err := cmd.Execute()

		require.NoError(t, err)
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
		cmd := command.CobraRoot()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tree", "--config-testcase", "namespace-scope", "--cluster", "testdata"})

		err := cmd.Execute()

		require.NoError(t, err)
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

		cmd := command.CobraRoot()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tree"})

		err := cmd.Execute()

		require.Error(t, err)
	})

	t.Run("empty source path", func(t *testing.T) {
		t.Parallel()

		cmd := command.CobraRoot()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tree", ""})

		err := cmd.Execute()

		require.Error(t, err)
	})

	t.Run("multi template config", func(t *testing.T) {
		t.Parallel()

		cmd := command.CobraRoot()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tree", "--config-path", "testdata/.config.yaml", "--config-testcase", "namespace-scope", "testdata"})

		err := cmd.Execute()

		require.Error(t, err)
	})

	t.Run("missing source", func(t *testing.T) {
		t.Parallel()

		cmd := command.CobraRoot()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tree", "invisible_chicken"})
		err := cmd.Execute()

		require.Error(t, err)
	})
}

func TestTree_ConfigPath(t *testing.T) {
	t.Parallel()

	t.Run("missing config path", func(t *testing.T) {
		t.Parallel()

		cmd := command.CobraRoot()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tree", "--config-path", "nonexistent", "testdata"})
		err := cmd.Execute()

		require.Error(t, err)
	})
}

func TestTree_ConfigTemplate(t *testing.T) {
	t.Parallel()

	t.Run("missing config path", func(t *testing.T) {
		t.Parallel()

		cmd := command.CobraRoot()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tree", "--config-testcase", "nonexistent", "testdata"})
		err := cmd.Execute()

		require.Error(t, err)
	})
}

func TestTree_NoConfig(t *testing.T) {
	t.Parallel()

	t.Run("namespace scoped", func(t *testing.T) {
		t.Parallel()

		cmd := command.CobraRoot()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tree", "testdata"})

		err := cmd.Execute()

		require.Error(t, err)
	})

	t.Run("cluster scoped", func(t *testing.T) {
		cmd := command.CobraRoot()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tree", "--cluster", "testdata"})

		err := cmd.Execute()

		require.Error(t, err)
	})
}

func TestTree_FileConfig(t *testing.T) {
	t.Parallel()

	t.Run("namespace scoped", func(t *testing.T) {
		t.Parallel()

		cmd := command.CobraRoot()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tree", "--config-path", "testdata/.config.yaml", "testdata"})

		err := cmd.Execute()

		require.NoError(t, err)
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
		cmd := command.CobraRoot()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tree", "--config-path", "testdata/.config.yaml", "--cluster", "testdata"})

		err := cmd.Execute()

		require.NoError(t, err)
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
