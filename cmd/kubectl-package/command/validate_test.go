package command_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"package-operator.run/package-operator/cmd/kubectl-package/command"
)

func TestValidateFolder(t *testing.T) {
	t.Parallel()

	cmd := command.CobraRoot()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"validate", "testdata"})

	err := cmd.Execute()

	require.Nil(t, err)
	require.Len(t, stdout.String(), 0)
	require.Len(t, stderr.String(), 0)
}
