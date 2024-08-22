package updatecmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"

	internalcmd "package-operator.run/internal/cmd"
)

func TestUpdate_WithoutPath(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(internalcmd.NewUpdate(internalcmd.WithLog{
		Log: logr.Logger{},
	}))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	require.Error(t, cmd.Execute())
	require.NotEmpty(t, stderr.String())
}

func TestUpdate_WithInvalidPath(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(internalcmd.NewUpdate(internalcmd.WithLog{
		Log: logr.Logger{},
	}))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"test-data"})

	require.Error(t, cmd.Execute())
	require.NotEmpty(t, stderr.String())
}

func TestUpdateWithValidPath_AlreadyUpdated(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(internalcmd.NewUpdate(internalcmd.WithLog{
		Log: logr.Logger{},
	}))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"testdata/up-to-date"})

	require.NoError(t, cmd.Execute())
	require.Empty(t, stderr.String())
	require.Equal(t, "Package is already up-to-date\n", stdout.String())
}

func TestUpdateWithValidPath_SuccessfullyUpdated(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(internalcmd.NewUpdate(internalcmd.WithLog{
		Log: logr.Logger{},
	}))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"testdata/successfully-updated"})

	require.NoError(t, cmd.Execute())
	require.Empty(t, stderr.String())
	require.Equal(t, "Package updated successfully!\n", stdout.String())

	const testLockFile = "testdata/successfully-updated/manifest.lock.yaml"

	_, err := os.Stat(testLockFile)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := os.Remove(testLockFile); err != nil {
			panic(err)
		}
	})
}
