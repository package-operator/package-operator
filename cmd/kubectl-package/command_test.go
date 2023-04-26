package main_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	kpkg "package-operator.run/package-operator/cmd/kubectl-package"
)

func TestRunSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stdin, stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	ret := kpkg.Run(ctx, stdin, stdout, stderr, []string{"version"})

	require.Equal(t, kpkg.ReturnCodeSuccess, ret)
}

func TestRunFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stdin, stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	ret := kpkg.Run(ctx, stdin, stdout, stderr, []string{"chicken"})

	require.Equal(t, kpkg.ReturnCodeError, ret)
}
