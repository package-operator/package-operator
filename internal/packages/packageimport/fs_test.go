package packageimport_test

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/packages/packageimport"
)

func TestFS(t *testing.T) {
	t.Parallel()
	ctx := logr.NewContext(context.Background(), testr.New(t))

	invalidEntries := packagecontent.Files{".git/chicken": {1, 2}, ".something": {3, 4}}
	validEntries := packagecontent.Files{"manifest.yaml": {5, 6}, "manifest.yml": {7, 8}, "subdir/somethingelse": {9, 10}}

	memFS := fstest.MapFS{}
	for k, v := range validEntries {
		memFS[k] = &fstest.MapFile{Data: v}
	}
	for k, v := range invalidEntries {
		memFS[k] = &fstest.MapFile{Data: v}
	}

	contents, err := packageimport.FS(ctx, memFS)
	require.Nil(t, err)
	assert.Equal(t, validEntries, contents)
}
