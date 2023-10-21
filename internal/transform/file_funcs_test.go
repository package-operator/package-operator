package transform

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testFiles = map[string][]byte{
	"t.yaml": []byte(`t: v`),
	"other":  nil,
}

func TestFileFuncs(t *testing.T) {
	t.Parallel()
	fm := FileFuncs(testFiles)
	assert.NotNil(t, fm["getFile"])
	assert.NotNil(t, fm["getFileGlob"])
}

func Test_getFile(t *testing.T) {
	t.Parallel()
	found, err := getFile(testFiles)("t.yaml")
	require.NoError(t, err)
	assert.Equal(t, `t: v`, found)

	_, err = getFile(testFiles)("xxx.yaml")
	require.ErrorIs(t, err, ErrFileNotFound)
}

func Test_getFileGlob(t *testing.T) {
	t.Parallel()
	found, err := getFileGlob(testFiles)("**.yaml")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"t.yaml": `t: v`,
	}, found)
}
