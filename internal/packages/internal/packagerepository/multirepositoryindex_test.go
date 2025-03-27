package packagerepository

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/empty"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/apis/manifests"
)

const (
	repoName                     = "hans"
	multiRepositoryIndexFileSeed = `---
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
)

func TestLoad(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	mri := NewMultiRepositoryIndex()
	require.NoError(t, mri.LoadRepository(ctx, strings.NewReader(multiRepositoryIndexFileSeed)))

	testMRI(ctx, t, mri)
}

func TestLoadError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	mri := NewMultiRepositoryIndex()
	require.Error(t, mri.LoadRepository(ctx, strings.NewReader("foobar")))
}

func TestLoadFromFile(t *testing.T) {
	t.Parallel()

	const hansRepoFile = "testdata/hans.repo.yaml"
	require.NoError(t,
		os.WriteFile(hansRepoFile, []byte(multiRepositoryIndexFileSeed), os.ModePerm))
	t.Cleanup(func() {
		if err := os.Remove(hansRepoFile); err != nil {
			panic(err)
		}
	})

	ctx := t.Context()
	mri := NewMultiRepositoryIndex()
	require.NoError(t,
		mri.LoadRepositoryFromFile(ctx, hansRepoFile))

	testMRI(ctx, t, mri)

	repo, err := mri.GetRepository("hans")
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll("testdata", os.ModePerm))

	file, err := os.Create(hansRepoFile)
	require.NoError(t, err)
	defer func() {
		if err := file.Close(); err != nil {
			panic(err)
		}
	}()

	require.NoError(t, repo.Export(ctx, file))
}

func TestLoadFromFileError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	mri := NewMultiRepositoryIndex()
	require.Error(t, mri.LoadRepositoryFromFile(ctx, "foobar.xyz"))
}

func TestLoadFromOCI(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	idx, err := LoadRepository(ctx, strings.NewReader(multiRepositoryIndexFileSeed))
	require.NoError(t, err)
	ociImg, err := SaveRepositoryToOCI(ctx, idx)
	require.NoError(t, err)

	mri := NewMultiRepositoryIndex()
	require.NoError(t, mri.LoadRepositoryFromOCI(ctx, ociImg))

	testMRI(ctx, t, mri)
}

func TestLoadFromOCIError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	mri := NewMultiRepositoryIndex()
	require.Error(t, mri.LoadRepositoryFromOCI(ctx, empty.Image))
}

func testMRI(ctx context.Context, t *testing.T, mri *MultiRepositoryIndex) {
	t.Helper()

	newEntry := entryFor(repoName, "pkg", "quay.io/xxx", "678", "v1.3.0")
	assert.Equal(t, "pkg."+repoName, newEntry.FQDN())

	// Read
	entry1, err := mri.GetVersion(repoName, "pkg", "v1.2.3")
	require.NoError(t, err)
	latest, err := mri.GetLatestEntry(repoName, "pkg")
	require.NoError(t, err)
	assert.Equal(t, entry1, latest)

	_, err = mri.GetVersion(repoName, "foo", "v1.2.3")
	require.Error(t, err)
	_, err = mri.GetLatestEntry(repoName, "foo")
	require.Error(t, err)

	_, err = mri.GetVersion("bar", "foo", "v1.2.3")
	require.Error(t, err)
	_, err = mri.GetLatestEntry("bar", "foo")
	require.Error(t, err)

	// Add entry 2
	require.NoError(t, mri.Add(ctx, newEntry))

	// Check data after adding entry 2
	latest, err = mri.GetLatestEntry(repoName, "pkg")
	require.NoError(t, err)
	assert.Equal(t, newEntry, latest)

	assert.Len(t, mri.ListEntries(repoName, "pkg"), 2)
	assert.Empty(t, mri.ListEntries(repoName, "foo"))
	assert.Len(t, mri.ListAllEntries(), 2)

	vs, err := mri.ListVersions(repoName, "pkg")
	require.NoError(t, err)
	assert.Len(t, vs, 3)
	_, err = mri.ListVersions(repoName, "foo")
	require.Error(t, err)

	byDigest, err := mri.GetDigest(repoName, "pkg", "12345")
	require.NoError(t, err)
	assert.Equal(t, entry1, byDigest)
	_, err = mri.GetDigest(repoName, "pkg", "67890")
	require.Error(t, err)
	_, err = mri.GetDigest(repoName, "foo", "67890")
	require.Error(t, err)

	require.NoError(t, mri.Remove(ctx, newEntry))

	otherRepo := "other"
	otherRepoEntry := entryFor("other", "pkg", "quay.io/yyy", "9AB", "v2.0.0")
	assert.Equal(t, "pkg.other", otherRepoEntry.FQDN())

	require.NoError(t, mri.Add(ctx, otherRepoEntry))

	assert.Len(t, mri.ListEntries(repoName, "pkg"), 1)
	_, err = mri.GetVersion(repoName, "pkg", "v1.2.3")
	require.NoError(t, err)
	_, err = mri.GetVersion(repoName, "pkg", "v2.0.0")
	require.Error(t, err)
	_, err = mri.GetDigest(repoName, "pkg", "12345")
	require.NoError(t, err)
	_, err = mri.GetDigest(repoName, "pkg", "9AB")
	require.Error(t, err)

	assert.Len(t, mri.ListEntries(otherRepo, "pkg"), 1)
	_, err = mri.GetVersion(otherRepo, "pkg", "v1.2.3")
	require.Error(t, err)
	_, err = mri.GetVersion(otherRepo, "pkg", "v2.0.0")
	require.NoError(t, err)
	_, err = mri.GetDigest(otherRepo, "pkg", "12345")
	require.Error(t, err)
	_, err = mri.GetDigest(otherRepo, "pkg", "9AB")
	require.NoError(t, err)

	assert.Len(t, mri.ListAllEntries(), 2)

	_, err = mri.GetRepository(otherRepo)
	require.NoError(t, err)

	require.NoError(t, mri.Remove(ctx, otherRepoEntry))

	_, err = mri.GetRepository(otherRepo)
	require.Error(t, err)

	assert.Len(t, mri.ListEntries(repoName, "pkg"), 1)
	assert.Empty(t, mri.ListEntries(otherRepo, "pkg"))
	assert.Len(t, mri.ListAllEntries(), 1)
}

func entryFor(repoName, name, image, digest, version string) Entry {
	entry := Entry{
		RepositoryEntry: &manifests.RepositoryEntry{
			Data: manifests.RepositoryEntryData{
				Name:     name,
				Image:    image,
				Digest:   digest,
				Versions: []string{version},
			},
		},
		RepositoryName: repoName,
	}
	return entry
}
