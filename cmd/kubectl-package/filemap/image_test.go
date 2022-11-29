package filemap_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"package-operator.run/package-operator/cmd/kubectl-package/filemap"
)

func TestFromToImage(t *testing.T) {
	t.Parallel()

	seedingFileMap := map[string][]byte{
		"manifest.yaml":        {5, 6},
		"manifest.yml":         {7, 8},
		"subdir/somethingelse": {9, 10},
	}

	image, err := filemap.ToImage(seedingFileMap)
	assert.Nil(t, err)

	reapedFileMap, err := filemap.FromImage(image)

	assert.Nil(t, err)
	assert.True(t, reflect.DeepEqual(reapedFileMap, seedingFileMap))

}
