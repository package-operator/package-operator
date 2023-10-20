package packageimport

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages/packagetypes"
)

func TestFromFS(t *testing.T) {
	t.Parallel()
	ctx := logr.NewContext(context.Background(), testr.New(t))

	validEntries := packagetypes.Files{"manifest.yaml": {5, 6}, "manifest.yml": {7, 8}, "subdir/somethingelse": {9, 10}}

	memFS := fstest.MapFS{}
	for k, v := range validEntries {
		memFS[k] = &fstest.MapFile{Data: v}
	}

	rawPkg, err := FromFS(ctx, memFS)
	require.Nil(t, err)
	assert.Equal(t, validEntries, rawPkg.Files)
}

func TestFromFolder(t *testing.T) {
	t.Parallel()
	ctx := logr.NewContext(context.Background(), testr.New(t))

	rawPkg, err := FromFolder(ctx, "testdata/fs")
	require.Nil(t, err)
	assert.Len(t, rawPkg.Files, 2)
	assert.Equal(t, "xxx\n", string(rawPkg.Files["my-stuff.txt"]))
	assert.Equal(t, "test: test\n", string(rawPkg.Files["file1.yaml"]))
}
