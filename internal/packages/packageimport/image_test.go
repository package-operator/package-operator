package packageimport_test

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
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

	image, err := packageexport.Image(seedingFiles)
	assert.Nil(t, err)

	reapedFiles, err := packageimport.Image(ctx, image)
	require.Nil(t, err)

	assert.Equal(t, packagecontent.Files{
		"manifest.yaml":        {5, 6},
		"manifest.yml":         {7, 8},
		"subdir/somethingelse": {9, 10},
	}, reapedFiles)
}

func TestHelmImage(t *testing.T) {
	t.Parallel()

	subFiles := map[string][]byte{
		"nginx/.tmp/xxx.yaml": []byte(`123`),
		"nginx/Chart.yaml":    []byte(`123`),
	}
	layer, err := crane.Layer(subFiles)
	require.NoError(t, err)

	image, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{})
	require.NoError(t, err)

	image, err = mutate.AppendLayers(image, layer)
	require.NoError(t, err)

	image, err = mutate.Canonical(image)
	require.NoError(t, err)

	ctx := context.Background()
	files, err := packageimport.HelmImage(ctx, image)
	require.NoError(t, err)

	assert.Equal(t, packagecontent.Files{
		"Chart.yaml": []byte(`123`),
	}, files)
}
