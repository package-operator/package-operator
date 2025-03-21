package packageimport

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromFolder(t *testing.T) {
	t.Parallel()
	ctx := logr.NewContext(t.Context(), testr.NewWithOptions(t, testr.Options{
		Verbosity: 99,
	}))

	rawPkg, err := FromFolder(ctx, "testdata/fs")
	require.NoError(t, err)
	assert.Len(t, rawPkg.Files, 2)
	assert.Equal(t, "xxx\n", string(rawPkg.Files["my-stuff.txt"]))
	assert.Equal(t, "test: test\n", string(rawPkg.Files["file1.yaml"]))
}
