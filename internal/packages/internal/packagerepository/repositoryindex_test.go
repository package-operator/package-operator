package packagerepository

import (
	"context"
	"os"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

const savedRepositoryExpectedContent = `---
apiVersion: manifests.package-operator.run/v1alpha1
kind: Repository
metadata:
  creationTimestamp: "2015-07-11T04:01:00Z"
  name: test-name
  namespace: test-namespace
---
apiVersion: manifests.package-operator.run/v1alpha1
data:
  digest: "67890"
  image: quay.io/yyy
  name: pkg
  versions:
  - v1.3.6
  - v1.3.5
kind: RepositoryEntry
metadata:
  name: pkg.67890
`

func TestLoadRepositoryFromFile(t *testing.T) {
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
	t.Cleanup(func() {
		if err := os.Remove(repoPath); err != nil {
			panic(err)
		}
	})

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
	assert.Len(t, ri.ListAllEntries(), 2)
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
	defer func() {
		if err := file.Close(); err != nil {
			panic(err)
		}
	}()

	require.NoError(t, ri.Export(ctx, file))
}

func TestSaveRepositoryToFile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo, err := repositoryToSave(ctx)
	require.NoError(t, err)

	const repoPath = "testdata/saved-repo.yaml"
	err = SaveRepositoryToFile(ctx, repoPath, repo)
	require.NoError(t, err)

	writtenBytes, err := os.ReadFile(repoPath)
	require.NoError(t, err)
	require.Equal(t, savedRepositoryExpectedContent, string(writtenBytes))

	t.Cleanup(func() {
		if err := os.Remove(repoPath); err != nil {
			panic(err)
		}
	})
}

func TestSaveAndLoadRepositoryToOCI(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo, err := repositoryToSave(ctx)
	require.NoError(t, err)

	image, err := SaveRepositoryToOCI(ctx, repo)
	require.NoError(t, err)

	hash, err := image.Digest()
	require.NoError(t, err)
	assert.Equal(t, "7a12dda67da105a2bed2792d62a6122b43ec4caf74db30440958218093efe802", hash.Hex)

	ri, err := LoadRepositoryFromOCI(ctx, image)
	require.NoError(t, err)

	assert.False(t, ri.IsEmpty())
	assert.Equal(t, "test-name", ri.Metadata().Name)

	entry1, err := ri.GetVersion("pkg", "v1.3.5")
	require.NoError(t, err)
	_, err = ri.GetVersion("pkg", "v2.0.0")
	require.Error(t, err)
	_, err = ri.GetVersion("foo", "v1.3.5")
	require.Error(t, err)

	latest, err := ri.GetLatestEntry("pkg")
	require.NoError(t, err)
	_, err = ri.GetLatestEntry("foo")
	require.Error(t, err)

	assert.Equal(t, entry1, latest)

	assert.Len(t, ri.ListEntries("pkg"), 1)
	assert.Empty(t, ri.ListEntries("foo"))
	assert.Len(t, ri.ListAllEntries(), 1)

	vrs, err := ri.ListVersions("pkg")
	require.NoError(t, err)
	assert.Len(t, vrs, 2)
	_, err = ri.ListVersions("foo")
	require.Error(t, err)

	vs, err := ri.ListVersions("pkg")
	require.NoError(t, err)
	assert.Len(t, vs, 2)
	byDigest, err := ri.GetDigest("pkg", "67890")
	require.NoError(t, err)
	assert.Equal(t, entry1, byDigest)
	_, err = ri.GetDigest("foo", "67890")
	require.Error(t, err)
}

func repositoryToSave(ctx context.Context) (*RepositoryIndex, error) {
	repo := NewRepositoryIndex(metav1.ObjectMeta{
		Name:              "test-name",
		Namespace:         "test-namespace",
		CreationTimestamp: metav1.Date(2015, time.July, 11, 4, 1, 0, 0, time.UTC),
	})
	err := repo.Add(ctx, &manifests.RepositoryEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "pkg.67890"},
		Data: manifests.RepositoryEntryData{
			Name:     "pkg",
			Image:    "quay.io/yyy",
			Digest:   "67890",
			Versions: []string{"v1.3.5", "v1.3.6"},
		},
	})
	return repo, err
}
