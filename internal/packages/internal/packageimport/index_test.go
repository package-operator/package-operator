package packageimport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathIndexMap(t *testing.T) {
	t.Parallel()
	p, err := Index("testdata/fs/")
	require.NoError(t, err)

	expected := map[string]struct{}{
		"my-stuff.txt":   {},
		"file1.yaml":     {},
		".dotdot":        {},
		"sub/.dotdot":    {},
		"incl/test.yaml": {},
	}
	assert.Equal(t, expected, p)
}
