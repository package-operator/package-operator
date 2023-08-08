package validatecmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	internalcmd "package-operator.run/internal/cmd"
)

func TestValidateFolder(t *testing.T) {
	t.Parallel()

	scheme, err := internalcmd.NewScheme()
	require.NoError(t, err)

	cmd := NewCmd(internalcmd.NewValidate(scheme))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"testdata"})

	require.Nil(t, cmd.Execute())
	require.Len(t, stdout.String(), 0)
	require.Len(t, stderr.String(), 0)
}
