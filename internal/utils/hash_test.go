package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

func TestComputeHash(t *testing.T) {
	testObj := struct{ name string }{name: "test"}

	t.Run("no collisions", func(t *testing.T) {
		hash := ComputeHash(testObj, nil)
		assert.Equal(t, "f8856fd5d", hash)
	})

	t.Run("with collisions", func(t *testing.T) {
		hash := ComputeHash(testObj, pointer.Int32(2))
		assert.Equal(t, "8697b5dc56", hash)
	})
}
