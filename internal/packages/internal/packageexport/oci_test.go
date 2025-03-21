package packageexport

import (
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages/internal/packagetypes"
	"package-operator.run/internal/testutil"
)

func TestToOCI(t *testing.T) {
	t.Parallel()

	seedingFileMap := map[string][]byte{
		"manifest.yaml":        {5, 6},
		"manifest.yml":         {7, 8},
		"subdir/somethingelse": {9, 10},
	}
	rawPkg := &packagetypes.RawPackage{
		Files: seedingFileMap,
	}

	image, err := ToOCI(rawPkg)
	require.NoError(t, err)
	layers, err := image.Layers()
	require.NoError(t, err)
	require.Len(t, layers, 1)
}

func TestToOCIFile(t *testing.T) { //nolint:paralleltest
	f, err := os.CreateTemp(t.TempDir(), "pko-*.tar.gz")
	require.NoError(t, err)

	defer func() { require.NoError(t, os.Remove(f.Name())) }()
	defer func() { require.NoError(t, f.Close()) }()

	rawPkg := &packagetypes.RawPackage{}
	err = ToOCIFile(f.Name(), []string{"chickens:oldest"}, rawPkg)
	require.NoError(t, err)

	i, err := tarball.ImageFromPath(f.Name(), nil)
	require.NoError(t, err)
	_, err = i.Manifest()
	require.NoError(t, err)
}

func TestToPushedOCI(t *testing.T) { //nolint:paralleltest
	ctx := t.Context()

	reg := testutil.NewInMemoryRegistry()

	ref := "chickens:oldest"
	seedingFileMap := map[string][]byte{
		"manifest.yaml":        {5, 6},
		"manifest.yml":         {7, 8},
		"subdir/somethingelse": {9, 10},
	}
	rawPkg := &packagetypes.RawPackage{
		Files: seedingFileMap,
	}

	err := ToPushedOCI(ctx, []string{ref}, rawPkg, reg.CraneOpt)
	require.NoError(t, err)

	_, err = crane.Pull(ref, reg.CraneOpt)
	require.NoError(t, err)
}
