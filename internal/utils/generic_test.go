package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContains(t *testing.T) {
	t.Run("strings", func(t *testing.T) {
		s := []string{"t1", "t2", "t3"}
		assert.True(t, Contains(s, "t1"))
		assert.False(t, Contains(s, "t4"))
	})

	t.Run("int", func(t *testing.T) {
		s := []int{1, 2, 3}
		assert.True(t, Contains(s, 1))
		assert.False(t, Contains(s, 4))
	})
}

func TestMergeKeysFrom(t *testing.T) {
	t.Run("nil base", func(t *testing.T) {
		r := MergeKeysFrom(nil, map[string]string{
			"x": "x",
		})
		assert.Equal(t, map[string]string{"x": "x"}, r)
	})

	t.Run("nil output", func(t *testing.T) {
		r := MergeKeysFrom(nil, map[string]string{})
		assert.Nil(t, r)
	})
}
