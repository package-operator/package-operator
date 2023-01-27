package updatecmd_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"package-operator.run/package-operator/cmd/kubectl-package/command"
)

func TestValidateFolder(t *testing.T) {
	t.Parallel()

	cmd := command.CobraRoot()
	cmd.SetArgs([]string{"update", "testdata"})

	err := cmd.Execute()

	require.Nil(t, err)
}
