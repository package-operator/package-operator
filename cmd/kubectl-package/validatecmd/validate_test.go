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

	require.NoError(t, cmd.Execute())
	require.Equal(t, "Package validated successfully!", stdout.String())
	require.Empty(t, stderr.String())
}

func TestValidate_NoPath(t *testing.T) {
	t.Parallel()

	scheme, err := internalcmd.NewScheme()
	require.NoError(t, err)

	cmd := NewCmd(internalcmd.NewValidate(scheme))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	require.Error(t, cmd.Execute())
	require.NotEmpty(t, stderr.String())
}

func TestValidate_InvalidPath(t *testing.T) {
	t.Parallel()

	scheme, err := internalcmd.NewScheme()
	require.NoError(t, err)

	cmd := NewCmd(internalcmd.NewValidate(scheme))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"test-data"})

	require.Error(t, cmd.Execute())
	require.NotEmpty(t, stderr.String())
}
