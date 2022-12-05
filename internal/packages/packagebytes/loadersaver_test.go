package packagebytes_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"package-operator.run/package-operator/internal/packages/packagebytes"
)

func TestFromToImage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	seedingFileMap := map[string][]byte{
		"manifest.yaml":        {5, 6},
		"manifest.yml":         {7, 8},
		"subdir/somethingelse": {9, 10},
	}

	l := packagebytes.NewLoader()
	s := packagebytes.NewSaver()

	image, err := s.ToImage(seedingFileMap)
	assert.Nil(t, err)

	reapedFileMap, err := l.FromImage(ctx, image)

	assert.Nil(t, err)
	assert.True(t, reflect.DeepEqual(reapedFileMap, seedingFileMap))
}
