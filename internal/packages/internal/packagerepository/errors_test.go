package packagerepository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotFoundError(t *testing.T) {
	t.Parallel()
	t.Run("only name", func(t *testing.T) {
		t.Parallel()
		notFound := newPackageNotFoundError("pkg")
		assert.Equal(t, `package "pkg" not found`, notFound.Error())
	})

	t.Run("name and version", func(t *testing.T) {
		t.Parallel()
		notFound := newPackageVersionNotFoundError("pkg", "v1.2.3")
		assert.Equal(t, `package "pkg" version "v1.2.3" not found`, notFound.Error())
	})

	t.Run("name and version", func(t *testing.T) {
		t.Parallel()
		notFound := newPackageDigestNotFoundError("pkg", "123")
		assert.Equal(t, `package "pkg" digest "123" not found`, notFound.Error())
	})
}
