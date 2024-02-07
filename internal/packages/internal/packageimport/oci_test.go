package packageimport

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/google/go-containerregistry/pkg/crane"
	containerregistrypkgv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages/internal/packagetypes"
)

func TestFromOCI(t *testing.T) {
	t.Parallel()

	image := buildImage(t, map[string][]byte{
		packagetypes.OCIPathPrefix + "/file.yaml":                    []byte(`test: test`),
		packagetypes.OCIPathPrefix + "/.test-fixtures/something.yml": {11, 12},

		// hidden files that need to be dropped
		packagetypes.OCIPathPrefix + "/.test-fixtures/.something.yml": {11, 12},
		"file.yaml": []byte(`test: test`),
		packagetypes.OCIPathPrefix + "/bla/.xxx/something.yml": {11, 12},
	})

	ctx := logr.NewContext(context.Background(), testr.New(t))
	rawPkg, err := FromOCI(ctx, image)
	require.NoError(t, err)

	assert.Equal(t, packagetypes.Files{
		"file.yaml":                    []byte(`test: test`),
		".test-fixtures/something.yml": {11, 12},
	}, rawPkg.Files)
}

func TestFromOCI_EmptyImage(t *testing.T) {
	t.Parallel()

	image := buildImage(t, map[string][]byte{})

	ctx := logr.NewContext(context.Background(), testr.New(t))
	_, err := FromOCI(ctx, image)
	require.EqualError(t, err, packagetypes.ErrEmptyPackage.Error())
}

func buildImage(t *testing.T, layerData map[string][]byte) containerregistrypkgv1.Image {
	t.Helper()

	configFile := &containerregistrypkgv1.ConfigFile{
		Config: containerregistrypkgv1.Config{},
		RootFS: containerregistrypkgv1.RootFS{Type: "layers"},
	}
	image, err := mutate.ConfigFile(empty.Image, configFile)
	require.NoError(t, err)

	layer, err := crane.Layer(layerData)
	require.NoError(t, err)

	image, err = mutate.AppendLayers(image, layer)
	require.NoError(t, err)

	image, err = mutate.Canonical(image)
	require.NoError(t, err)

	return image
}
