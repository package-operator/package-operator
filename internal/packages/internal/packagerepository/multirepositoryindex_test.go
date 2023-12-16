package packagerepository

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/apis/manifests"
)

const multiRepositoryIndexFileSeed = `---
apiVersion: manifests.package-operator.run/v1alpha1
kind: Repository
metadata:
  creationTimestamp: "2023-11-22T09:24:01Z"
  name: hans
---
apiVersion: manifests.package-operator.run/v1alpha1
data:
  digest: "12345"
  image: quay.io/package-operator/xxx
  name: pkg
  versions:
  - v1.2.4
  - v1.2.3
kind: RepositoryEntry
metadata:
  creationTimestamp: null
  name: pkg.12345
`

func TestMultiRepositoryIndex(t *testing.T) {
	t.Parallel()

	const (
		pkgName      = "pkg"
		repo1Name    = "hans"
		hansRepoFile = "testdata/hans.repo.yaml"
	)
	require.NoError(t,
		os.WriteFile(hansRepoFile, []byte(multiRepositoryIndexFileSeed), os.ModePerm))
	t.Cleanup(func() { os.Remove(hansRepoFile) })

	entry2 := Entry{
		RepositoryEntry: &manifests.RepositoryEntry{
			Data: manifests.RepositoryEntryData{
				Name:     "pkg",
				Image:    "quay.io/xxx",
				Digest:   "678",
				Versions: []string{"v1.3.0"},
			},
		},
		RepositoryName: repo1Name,
	}
	assert.Equal(t, "pkg.hans", entry2.FQDN())

	mri := NewMultiRepositoryIndex()

	ctx := context.Background()
	require.NoError(t,
		mri.LoadRepositoryFromFile(ctx, hansRepoFile))

	// Read
	entry1, err := mri.GetVersion(repo1Name, "pkg", "v1.2.3")
	require.NoError(t, err)
	latest, err := mri.GetLatestEntry(repo1Name, "pkg")
	require.NoError(t, err)
	assert.Equal(t, entry1, latest)

	// Add entry 2
	require.NoError(t, mri.Add(ctx, entry2))

	// Check data after adding entry 2
	latest, err = mri.GetLatestEntry(repo1Name, "pkg")
	require.NoError(t, err)
	assert.Equal(t, entry2, latest)
	assert.Len(t, mri.ListEntries(repo1Name, "pkg"), 2)
	vs, err := mri.ListVersions(repo1Name, "pkg")
	require.NoError(t, err)
	assert.Len(t, vs, 3)
	byDigest, err := mri.GetDigest(repo1Name, "pkg", "12345")
	require.NoError(t, err)
	assert.Equal(t, entry1, byDigest)

	require.NoError(t, mri.Remove(ctx, entry2))

	repo, err := mri.GetRepository("hans")
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll("testdata", os.ModePerm))
	file, err := os.Create(hansRepoFile)
	require.NoError(t, err)
	defer file.Close()

	require.NoError(t, repo.Export(ctx, file))
}
