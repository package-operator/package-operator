package command_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"package-operator.run/package-operator/cmd/kubectl-package/command"
)

func TestRunSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stdin, stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	ret := command.Run(ctx, stdin, stdout, stderr, []string{"version"})

	require.Equal(t, command.ReturnCodeSuccess, ret)
}

func TestRunFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stdin, stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	ret := command.Run(ctx, stdin, stdout, stderr, []string{"chicken"})

	require.Equal(t, command.ReturnCodeError, ret)
}
