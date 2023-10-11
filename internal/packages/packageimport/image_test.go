package packageimport_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages/packagecontent"
	"package-operator.run/internal/packages/packageexport"
	"package-operator.run/internal/packages/packageimport"
)

func TestImageLoadSave(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	seedingFiles := packagecontent.Files{
		"manifest.yaml":        {5, 6},
		"manifest.yml":         {7, 8},
		"subdir/somethingelse": {9, 10},
		// hidden files that need to be dropped
		".test-fixtures/something.yml":  {11, 12},
		".test-fixtures/.something.yml": {11, 12},
		"bla/.xxx/something.yml":        {11, 12},
	}

	image, err := packageexport.Image(seedingFiles, packagecontent.Metadata{})
	assert.Nil(t, err)

	reapedFiles, err := packageimport.Image(ctx, image)
	require.Nil(t, err)

	assert.Equal(t, packagecontent.Files{
		"manifest.yaml":        {5, 6},
		"manifest.yml":         {7, 8},
		"subdir/somethingelse": {9, 10},
	}, reapedFiles)
}
