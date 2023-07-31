package versioncmd

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCobraVersion(t *testing.T) {
	t.Parallel()

	cmd := NewCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{})

	require.Nil(t, cmd.Execute())
	require.Len(t, stderr.String(), 0)
	require.Contains(t, stdout.String(), runtime.Version())
}
