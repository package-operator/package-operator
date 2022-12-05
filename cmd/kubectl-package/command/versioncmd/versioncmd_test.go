package versioncmd_test

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"package-operator.run/package-operator/cmd/kubectl-package/command"
)

func TestCobraVersion(t *testing.T) {
	cmd := command.CobraRoot()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"version", "--embedded"})

	require.Nil(t, cmd.Execute())
	require.Len(t, stderr.String(), 0)
	require.Contains(t, stdout.String(), runtime.Version())
}
