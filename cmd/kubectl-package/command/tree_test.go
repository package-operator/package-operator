package command_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/package-operator/cmd/kubectl-package/command"
)

func TestTree(t *testing.T) {
	t.Parallel()

	t.Run("namespace scoped", func(t *testing.T) {
		cmd := command.CobraRoot()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tree", "testdata"})

		err := cmd.Execute()

		require.Nil(t, err)
		require.Len(t, stderr.String(), 0)

		const expectedOutput = `test-stub
Package <namespace>/<name>
└── Phase deploy
    └── apps/v1, Kind=Deployment /test-stub-<name>
`
		assert.Equal(t, expectedOutput, stdout.String())
	})

	t.Run("cluster scoped", func(t *testing.T) {
		cmd := command.CobraRoot()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tree", "testdata", "--cluster"})

		err := cmd.Execute()

		require.Nil(t, err)
		require.Len(t, stderr.String(), 0)

		const expectedOutput = `test-stub
ClusterPackage /<name>
└── Phase namespace
│   ├── /v1, Kind=Namespace /<name>
└── Phase deploy
    └── apps/v1, Kind=Deployment <name>/test-stub-<name>
`
		assert.Equal(t, expectedOutput, stdout.String())
	})
}
