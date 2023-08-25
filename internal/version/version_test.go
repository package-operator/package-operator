package version_test

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"package-operator.run/internal/version"
)

func TestGet(t *testing.T) {
	t.Parallel()

	info := version.Get()

	// Test that the embedded version info is actually filled
	require.Equal(t, info.GoVersion, runtime.Version())
}

func TestWriteWithoutAppVersion(t *testing.T) {
	t.Parallel()

	info := version.Get()

	buf := &bytes.Buffer{}
	err := info.Write(buf)

	require.NoError(t, err)
	require.Contains(t, buf.String(), "go\t")
}

func TestWriteWithAppVersion(t *testing.T) {
	t.Parallel()

	info := version.Get()
	info.ApplicationVersion = "cheese"

	buf := &bytes.Buffer{}
	err := info.Write(buf)

	require.NoError(t, err)
	require.Contains(t, buf.String(), "pko\t")
}
