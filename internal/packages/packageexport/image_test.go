package packageexport_test

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages/packageexport"
	"package-operator.run/internal/testutil"
)

func TestImage(t *testing.T) {
	t.Parallel()

	seedingFileMap := map[string][]byte{"manifest.yaml": {5, 6}, "manifest.yml": {7, 8}, "subdir/somethingelse": {9, 10}}

	image, err := packageexport.Image(seedingFileMap)
	require.Nil(t, err)
	layers, err := image.Layers()
	require.Nil(t, err)
	require.Len(t, layers, 1)
}

func TestPushedImage(t *testing.T) { //nolint:paralleltest
	ctx := context.Background()

	reg := testutil.NewInMemoryRegistry()

	ref := "chickens:oldest"
	seedingFileMap := map[string][]byte{"manifest.yaml": {5, 6}, "manifest.yml": {7, 8}, "subdir/somethingelse": {9, 10}}

	err := packageexport.PushedImage(ctx, []string{ref}, seedingFileMap, reg.CraneOpt)
	require.NoError(t, err)

	_, err = crane.Pull(ref, reg.CraneOpt)
	require.NoError(t, err)
}
