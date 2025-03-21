package packageimport

import (
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

	image := testutil.BuildImage(t, map[string][]byte{
		packagetypes.OCIPathPrefix + "/file.yaml": []byte(`test: test`),

		// hidden files that need to be dropped
		"file.yaml": []byte(`test: test`),
		packagetypes.OCIPathPrefix + "/.test-fixtures/something.yml":  {11, 12},
		packagetypes.OCIPathPrefix + "/.test-fixtures/.something.yml": {11, 12},
		packagetypes.OCIPathPrefix + "/bla/.xxx/something.yml":        {11, 12},
	})

	ctx := logr.NewContext(t.Context(), testr.New(t))
	rawPkg, err := FromOCI(ctx, image)
	require.NoError(t, err)

	assert.Equal(t, packagetypes.Files{
		"file.yaml": []byte(`test: test`),
	}, rawPkg.Files)
}

func TestFromOCI_EmptyImage(t *testing.T) {
	t.Parallel()

	image := testutil.BuildImage(t, map[string][]byte{})

	ctx := logr.NewContext(t.Context(), testr.New(t))
	_, err := FromOCI(ctx, image)
	require.EqualError(t, err, packagetypes.ErrEmptyPackage.Error())
}
