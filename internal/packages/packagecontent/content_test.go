package packagecontent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFiles_DeepCopy(t *testing.T) {
	t.Parallel()

	f := Files{"test": []byte("xxx")}

	newF := f.DeepCopy()
	assert.NotSame(t, f, newF)                 // new map
	assert.NotSame(t, f["test"], newF["test"]) // new slice
	assert.Equal(t, f, newF)                   // equal content
}
