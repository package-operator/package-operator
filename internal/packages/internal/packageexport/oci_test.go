package packageexport

import (
	"context"
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
	f, err := os.CreateTemp("", "pko-*.tar.gz")
	require.NoError(t, err)

	defer func() { require.NoError(t, os.Remove(f.Name())) }() //nolint:gosec // G703: Safe - path is from os.CreateTemp
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
	ctx := context.Background()

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

	digest, err := ToPushedOCI(ctx, []string{ref}, rawPkg, reg.CraneOpt)
	require.NoError(t, err)
	require.Equal(t, "sha256:8012c104de49c1b7d05db3ed1033f41a9cff0d0f52f74dbcf9622ecf20a44c61", digest)

	_, err = crane.Pull(ref, reg.CraneOpt)
	require.NoError(t, err)
}
