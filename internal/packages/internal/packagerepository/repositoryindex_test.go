package packagerepository

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"package-operator.run/internal/apis/manifests"
)

func TestRepositoryIndex(t *testing.T) {
	t.Parallel()
	entry2 := &manifests.RepositoryEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pkg",
		},
		Data: manifests.RepositoryEntryData{
			Image:    "quay.io/xxx",
			Digest:   "678",
			Versions: []string{"v1.3.0"},
		},
	}

	const repoPath = "testdata/repo.yaml"
	ctx := context.Background()

	ri, err := LoadRepositoryFromFile(ctx, repoPath)
	require.NoError(t, err)

	entry1, err := ri.GetVersion("pkg", "v1.2.3")
	require.NoError(t, err)
	latest, err := ri.GetLatestEntry("pkg")
	require.NoError(t, err)
	assert.Equal(t, entry1, latest)

	require.NoError(t, ri.Add(ctx, entry2))
	latest, err = ri.GetLatestEntry("pkg")
	require.NoError(t, err)
	assert.Equal(t, entry2, latest)

	assert.Len(t, ri.ListEntries("pkg"), 2)
	assert.Len(t, ri.ListVersions("pkg"), 3)
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
