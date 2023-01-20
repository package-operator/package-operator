package packageimport_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/packages/packageexport"
	"package-operator.run/package-operator/internal/packages/packageimport"
)

func TestImageLoadSave(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	seedingFiles := packagecontent.Files{"manifest.yaml": {5, 6}, "manifest.yml": {7, 8}, "subdir/somethingelse": {9, 10}}

	image, err := packageexport.Image(seedingFiles)
	assert.Nil(t, err)

	reapedFiles, err := packageimport.Image(ctx, image)

	assert.Nil(t, err)
	assert.True(t, reflect.DeepEqual(reapedFiles, seedingFiles))
}
