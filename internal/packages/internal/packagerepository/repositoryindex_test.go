package packagerepository

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/apis/manifests"
)

const repositoryIndexFileSeed = `---
apiVersion: manifests.package-operator.run/v1alpha1
kind: Repository
metadata:
  creationTimestamp: "2023-11-22T09:24:01Z"
  name: test
---
apiVersion: manifests.package-operator.run/v1alpha1
data:
  digest: "12345"
  image: quay.io/xxx
  name: pkg
  versions:
  - v1.2.4
  - v1.2.3
kind: RepositoryEntry
metadata:
  creationTimestamp: null
  name: pkg.12345
`

func TestRepositoryIndex(t *testing.T) {
	t.Parallel()
	entry2 := &manifests.RepositoryEntry{
		Data: manifests.RepositoryEntryData{
			Name:     "pkg",
			Image:    "quay.io/xxx",
			Digest:   "678",
			Versions: []string{"v1.3.0"},
		},
	}

	const repoPath = "testdata/repo.yaml"
	require.NoError(t,
		os.WriteFile(repoPath, []byte(repositoryIndexFileSeed), os.ModePerm))
	t.Cleanup(func() { os.Remove(repoPath) })

	ctx := context.Background()

	ri, err := LoadRepositoryFromFile(ctx, repoPath)
	require.NoError(t, err)

	// Check loaded data
	entry1, err := ri.GetVersion("pkg", "v1.2.3")
	require.NoError(t, err)
	latest, err := ri.GetLatestEntry("pkg")
	require.NoError(t, err)
	assert.Equal(t, entry1, latest)

	// Add entry 2
	require.NoError(t, ri.Add(ctx, entry2))

	// Check data after adding entry 2
	latest, err = ri.GetLatestEntry("pkg")
	require.NoError(t, err)
	assert.Equal(t, entry2, latest)
	assert.Len(t, ri.ListEntries("pkg"), 2)
	vs, err := ri.ListVersions("pkg")
	require.NoError(t, err)
	assert.Len(t, vs, 3)
	byDigest, err := ri.GetDigest("pkg", "12345")
	require.NoError(t, err)
	assert.Equal(t, entry1, byDigest)

	require.NoError(t, ri.Remove(ctx, entry2))

	require.NoError(t, os.MkdirAll("testdata", os.ModePerm))
	file, err := os.Create("testdata/repo.yaml")
	require.NoError(t, err)
	defer file.Close()

	require.NoError(t, ri.Export(ctx, file))
}
