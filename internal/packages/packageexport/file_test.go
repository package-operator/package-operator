package packageexport_test

import (
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/assert"

	"package-operator.run/internal/packages/packagecontent"
	"package-operator.run/internal/packages/packageexport"
)

func TestFile(t *testing.T) { //nolint:paralleltest
	f, err := os.CreateTemp("", "pko-*.tar.gz")
	assert.Nil(t, err)

	defer func() { assert.Nil(t, os.Remove(f.Name())) }()
	defer func() { assert.Nil(t, f.Close()) }()

	err = packageexport.File(f.Name(), []string{"chickens:oldest"}, packagecontent.Files{})
	assert.Nil(t, err)

	i, err := tarball.ImageFromPath(f.Name(), nil)
	assert.Nil(t, err)
	_, err = i.Manifest()
	assert.Nil(t, err)
}
