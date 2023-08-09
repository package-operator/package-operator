package version_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"package-operator.run/internal/version"
)

func TestGet(t *testing.T) {
	t.Parallel()

	info := version.Get()

	// Test that the embedded version info is actually filled
	assert.Equal(t, info.GoVersion, runtime.Version())
}
