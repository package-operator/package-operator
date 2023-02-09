package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

func TestComputeSHA256Hash(t *testing.T) {
	testObj := struct{ name string }{name: "test"}

	t.Run("no collisions", func(t *testing.T) {
		hash := ComputeSHA256Hash(testObj, nil)
		assert.Equal(t, "8931b36a81195ec09f6ba1c7766fbc95d41fc6e17ee11b0b5baa67cfcb5c072e", hash)
	})

	t.Run("with collisions", func(t *testing.T) {
		hash := ComputeSHA256Hash(testObj, pointer.Int32(2))
		assert.Equal(t, "21f3e24e03abf1c35cbafa23fbc9a4d200066c911ac1070539591639dc1500e2", hash)
	})
}

func TestComputeFNV32Hash(t *testing.T) {
	testObj := struct{ name string }{name: "test"}

	t.Run("no collisions", func(t *testing.T) {
		hash := ComputeFNV32Hash(testObj, nil)
		assert.Equal(t, "f8856fd5d", hash)
	})

	t.Run("with collisions", func(t *testing.T) {
		hash := ComputeFNV32Hash(testObj, pointer.Int32(2))
		assert.Equal(t, "8697b5dc56", hash)
	})
}
