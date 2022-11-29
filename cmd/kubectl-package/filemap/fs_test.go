package filemap_test

import (
	"context"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"

	"package-operator.run/package-operator/cmd/kubectl-package/filemap"
)

func TestFromFS(t *testing.T) {
	t.Parallel()
	ctx := logr.NewContext(context.Background(), testr.New(t))

	invalidEntries := map[string][]byte{
		".git/chicken": {1, 2},
		".something":   {3, 4},
	}

	validEntries := map[string][]byte{
		"manifest.yaml":        {5, 6},
		"manifest.yml":         {7, 8},
		"subdir/somethingelse": {9, 10},
	}

	memFS := fstest.MapFS{}

	for k, v := range validEntries {
		memFS[k] = &fstest.MapFile{Data: v}
	}

	for k, v := range invalidEntries {
		memFS[k] = &fstest.MapFile{Data: v}
	}

	fileMap, err := filemap.FromFS(ctx, memFS)

	assert.Nil(t, err)
	assert.True(t, reflect.DeepEqual(validEntries, fileMap))
}
