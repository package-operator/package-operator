package command_test

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"package-operator.run/package-operator/cmd/kubectl-package/command"
)

func TestCobraVersion(t *testing.T) {
	cmd := command.CobraRoot()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"version", "--embedded"})

	assert.Nil(t, cmd.Execute())
	assert.Len(t, stderr.String(), 0)
	assert.Contains(t, stdout.String(), runtime.Version())
}
