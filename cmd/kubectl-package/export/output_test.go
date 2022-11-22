package export_test

import (
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/assert"
	"package-operator.run/package-operator/cmd/kubectl-package/export"
)

func TestOutput(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp("", "pko-*.tar.gz")
	assert.Nil(t, err)

	defer func() { assert.Nil(t, os.Remove(f.Name())) }()
	defer func() { assert.Nil(t, f.Close()) }()

	ref := name.MustParseReference("chickens:oldest")

	err = export.ComressedTarToDisk(f.Name(), []name.Reference{ref}, empty.Image)
	assert.Nil(t, err)

	i, err := tarball.ImageFromPath(f.Name(), nil)
	assert.Nil(t, err)
	_, err = i.Manifest()
	assert.Nil(t, err)
}
