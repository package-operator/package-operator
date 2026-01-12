package packageimport

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages/internal/packagetypes"
	"package-operator.run/internal/testutil"
)

func TestFromOCI(t *testing.T) {
	t.Parallel()

	labels := map[string]string{"test": "test123"}
	image := testutil.BuildImage(t, map[string][]byte{
		packagetypes.OCIPathPrefix + "/file.yaml": []byte(`test: test`),

		// hidden files that need to be dropped
		"file.yaml": []byte(`test: test`),
		packagetypes.OCIPathPrefix + "/.test-fixtures/something.yml":  {11, 12},
		packagetypes.OCIPathPrefix + "/.test-fixtures/.something.yml": {11, 12},
		packagetypes.OCIPathPrefix + "/bla/.xxx/something.yml":        {11, 12},
	}, labels)

	ctx := logr.NewContext(context.Background(), testr.New(t))
	rawPkg, err := FromOCI(ctx, image)
	require.NoError(t, err)

	assert.Equal(t, packagetypes.Files{
		"file.yaml": []byte(`test: test`),
	}, rawPkg.Files)
	assert.Equal(t, labels, rawPkg.Labels)
}

func TestFromOCI_EmptyImage(t *testing.T) {
	t.Parallel()

	image := testutil.BuildImage(t, map[string][]byte{}, nil)

	ctx := logr.NewContext(context.Background(), testr.New(t))
	_, err := FromOCI(ctx, image)
	require.EqualError(t, err, packagetypes.ErrEmptyPackage.Error())
}
