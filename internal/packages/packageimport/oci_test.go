package packageimport

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/google/go-containerregistry/pkg/crane"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages/packagetypes"
)

func TestFromOCI(t *testing.T) {
	t.Parallel()
	configFile := &crv1.ConfigFile{
		Config: crv1.Config{},
		RootFS: crv1.RootFS{Type: "layers"},
	}
	image, err := mutate.ConfigFile(empty.Image, configFile)
	require.NoError(t, err)

	layer, err := crane.Layer(map[string][]byte{
		packagetypes.OCIPathPrefix + "/file.yaml": []byte(`test: test`),

		// hidden files that need to be dropped
		"file.yaml": []byte(`test: test`),
		packagetypes.OCIPathPrefix + "/.test-fixtures/something.yml":  {11, 12},
		packagetypes.OCIPathPrefix + "/.test-fixtures/.something.yml": {11, 12},
		packagetypes.OCIPathPrefix + "/bla/.xxx/something.yml":        {11, 12},
	})
	require.NoError(t, err)

	image, err = mutate.AppendLayers(image, layer)
	require.NoError(t, err)

	image, err = mutate.Canonical(image)
	require.NoError(t, err)

	ctx := logr.NewContext(context.Background(), testr.New(t))
	rawPkg, err := FromOCI(ctx, image)
	require.NoError(t, err)

	assert.Equal(t, packagetypes.Files{
		"file.yaml": []byte(`test: test`),
	}, rawPkg.Files)
}
