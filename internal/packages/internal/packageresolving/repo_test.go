package packageresolving

import (
	"context"
	"os"
	"reflect"
	"testing"

	"package-operator.run/internal/packages/internal/packagerepository"

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

func TestLoadRepo(t *testing.T) {
	t.Parallel()

	const repoPath = "testdata/repo.yaml"
	require.NoError(t,
		os.WriteFile(repoPath, []byte(repositoryIndexFileSeed), os.ModePerm))
	t.Cleanup(func() {
		if err := os.Remove(repoPath); err != nil {
			panic(err)
		}
	})

	tests := []struct {
		name    string
		repos   []manifests.PackageManifestRepository
		expSize int
		expErr  string
	}{
		{
			name:    "valid load from file",
			repos:   []manifests.PackageManifestRepository{{File: repoPath}},
			expSize: 1,
		},
		{
			name:   "non existing file",
			repos:  []manifests.PackageManifestRepository{{File: "testdata/foobar.yaml"}},
			expErr: "open testdata/foobar.yaml: no such file or directory",
		},
		{
			name:   "non existing container image",
			repos:  []manifests.PackageManifestRepository{{Image: "quay.io/package-operator/foobar:vX.Y.Z"}},
			expErr: "pull repository image",
		},
		{
			name:   "invalid container image",
			repos:  []manifests.PackageManifestRepository{{Image: "quay.io/package-operator/test-stub:v1.9.3"}},
			expErr: "read from image tar: EOF",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			mri, err := loadRepo(ctx, test.repos)
			if test.expErr == "" {
				require.NoError(t, err)
				assert.Len(t, mri.ListAllEntries(), test.expSize)
			} else {
				require.ErrorContains(t, err, test.expErr)
			}
		})
	}
}

func TestDefaultRepoLoader(t *testing.T) {
	t.Parallel()

	rl := defaultRepoLoaderIfNil(nil)
	assert.Equal(t, reflect.ValueOf(RepoLoader(loadRepo)).Pointer(), reflect.ValueOf(rl).Pointer())

	customLoader := func(
		context.Context, []manifests.PackageManifestRepository,
	) (*packagerepository.MultiRepositoryIndex, error) {
		return packagerepository.NewMultiRepositoryIndex(), nil
	}
	rl = defaultRepoLoaderIfNil(customLoader)
	assert.Equal(t, reflect.ValueOf(customLoader).Pointer(), reflect.ValueOf(rl).Pointer())
}
