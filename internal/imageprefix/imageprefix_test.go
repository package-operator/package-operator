package imageprefix

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMostSpecificOverride(t *testing.T) {
	t.Parallel()

	overrides := []Override{
		{From: "quay.io/original/", To: "quay.io/mirror/"},
		{From: "quay.io/original/foo", To: "quay.io/mirror/bar"},
		{From: "quay.io/original/f", To: "quay.io/mirror/baz"},
	}

	originalImage := "quay.io/original/foo:tag"

	overridden := Replace(originalImage, overrides)
	assert.Equal(t, "quay.io/mirror/bar:tag", overridden)
}

func TestNoAppliedOverride(t *testing.T) {
	t.Parallel()

	overrides := []Override{
		{From: "quay.io/notmatching/", To: "quay.io/mirror/"},
	}

	originalImage := "quay.io/original/foo:tag"

	overridden := Replace(originalImage, overrides)
	assert.Equal(t, "quay.io/original/foo:tag", overridden)
}
