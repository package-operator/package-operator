package validatecmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateFolder(t *testing.T) {
	t.Parallel()

	cmd := NewCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"testdata"})

	err := cmd.Execute()

	require.Nil(t, err)
	require.Len(t, stdout.String(), 0)
	require.Len(t, stderr.String(), 0)
}
