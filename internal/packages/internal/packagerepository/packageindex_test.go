package packagerepository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/apis/manifests"
)

func Test_packageIndex(t *testing.T) {
	t.Parallel()
	const pkgName = "pkg"
	entry1 := &manifests.RepositoryEntry{
		Data: manifests.RepositoryEntryData{
			Name:   pkgName,
			Image:  "quay.io/package-operator/xxx",
			Digest: "12345",
			Versions: []string{
				"v1.2.3", "v1.2.4",
			},
		},
	}

	pi := newPackageIndex(pkgName)

	assert.Equal(t, pkgName, pi.GetName())

	// Index empty
	assertEmptyPackageIndex(t, pi)

	// Add something
	ctx := t.Context()
	require.NoError(t, pi.Add(ctx, entry1))

	// Index populated
	assert.False(t, pi.IsEmpty(), "Is not empty")

	entry, err := pi.GetLatestEntry()
	require.NoError(t, err)
	assert.Equal(t, entry1, entry)

	entry, err = pi.GetVersion("v1.2.3")
	require.NoError(t, err)
	assert.Equal(t, entry1, entry)

	entry, err = pi.GetVersion("v1.2.4")
	require.NoError(t, err)
	assert.Equal(t, entry1, entry)

	entry, err = pi.GetDigest("12345")
	require.NoError(t, err)
	assert.Equal(t, entry1, entry)

	vs := pi.ListVersions()
	assert.Len(t, vs, 2)
	assert.Equal(t, []string{"v1.2.4", "v1.2.3"}, vs)
	assert.Len(t, pi.ListEntries(), 1)

	// Remove it again
	require.NoError(t, pi.Remove(ctx, entry1))
	assertEmptyPackageIndex(t, pi)
}

func assertEmptyPackageIndex(t *testing.T, pi *packageIndex) {
	t.Helper()

	assert.True(t, pi.IsEmpty(), "Is empty")
	_, err := pi.GetLatestEntry()
	require.EqualError(t, err, `package "pkg" not found`)
	_, err = pi.GetVersion("v1.2.3")
	require.EqualError(t, err, `package "pkg" version "v1.2.3" not found`)
	_, err = pi.GetDigest("123")
	require.EqualError(t, err, `package "pkg" digest "123" not found`)
	assert.Empty(t, pi.ListVersions())
	assert.Empty(t, pi.ListEntries())
}
