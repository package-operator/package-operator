package resolvebuild_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages"
	"package-operator.run/internal/packages/resolving/resolvebuild"
)

func TestResolveNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pkg := &manifests.PackageManifest{}
	idx := packages.NewMultiRepositoryIndex()
	r := resolvebuild.Resolver{Loader: func(ctx context.Context, pmr []manifests.PackageManifestRepository) (*packages.MultiRepositoryIndex, error) {
		return idx, nil
	}}
	a, err := r.ResolveBuild(ctx, pkg)

	require.NoError(t, err)
	require.Empty(t, a)
}

func TestResolveBasic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pkg := &manifests.PackageManifest{
		ObjectMeta: metav1.ObjectMeta{Name: "bigpkg"},
		Spec: manifests.PackageManifestSpec{
			Constraints:  []manifests.PackageManifestConstraint{},
			Repositories: []manifests.PackageManifestRepository{},
			Dependencies: []manifests.PackageManifestDependency{
				{Image: &manifests.PackageManifestDependencyImage{
					Name:    "döpöndöncö",
					Package: "bread.repo",
				}},
			},
		},
	}

	idx := packages.NewMultiRepositoryIndex()

	require.NoError(t, idx.Add(ctx, packages.Entry{
		RepositoryName: "repo",
		RepositoryEntry: &manifests.RepositoryEntry{
			Data: manifests.RepositoryEntryData{
				Image:    "image",
				Digest:   "digest",
				Name:     "bread",
				Versions: []string{"1.0.0", "2.0.0", "3.0.0"},
				Constraints: []manifests.PackageManifestConstraint{{
					UniqueInScope: &manifests.PackageManifestUniqueInScopeConstraint{},
				}},
			},
		},
	}))

	r := resolvebuild.Resolver{Loader: func(ctx context.Context, pmr []manifests.PackageManifestRepository) (*packages.MultiRepositoryIndex, error) {
		return idx, nil
	}}
	a, err := r.ResolveBuild(ctx, pkg)

	require.NoError(t, err)
	require.Len(t, a, 1)
	expected := manifests.PackageManifestLockDependency{
		Name:    "döpöndöncö",
		Image:   "image",
		Digest:  "digest",
		Version: "3.0.0",
	}
	require.Equal(t, expected, a[0])
}

func TestResolvePlatform(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pkg := &manifests.PackageManifest{
		ObjectMeta: metav1.ObjectMeta{Name: "bigpkg"},
		Spec: manifests.PackageManifestSpec{
			Constraints: []manifests.PackageManifestConstraint{
				{PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{Name: "platform", Range: ">=3"}},
			},
			Repositories: []manifests.PackageManifestRepository{},
			Dependencies: []manifests.PackageManifestDependency{
				{Image: &manifests.PackageManifestDependencyImage{
					Name:    "döpöndöncö",
					Package: "bread.repo",
				}},
			},
		},
	}

	idx := packages.NewMultiRepositoryIndex()

	require.NoError(t, idx.Add(ctx, packages.Entry{
		RepositoryName: "repo",
		RepositoryEntry: &manifests.RepositoryEntry{
			Data: manifests.RepositoryEntryData{
				Image:    "image",
				Digest:   "digest",
				Name:     "bread",
				Versions: []string{"3.0.0", "2.0.0", "1.0.0"},
				Constraints: []manifests.PackageManifestConstraint{{
					UniqueInScope:   &manifests.PackageManifestUniqueInScopeConstraint{},
					PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{Name: "platform", Range: ">=2"},
				}},
			},
		},
	}))

	r := resolvebuild.Resolver{Loader: func(ctx context.Context, pmr []manifests.PackageManifestRepository) (*packages.MultiRepositoryIndex, error) {
		return idx, nil
	}}
	a, err := r.ResolveBuild(ctx, pkg)

	require.NoError(t, err)
	require.Len(t, a, 1)
	expected := manifests.PackageManifestLockDependency{
		Name:    "döpöndöncö",
		Image:   "image",
		Digest:  "digest",
		Version: "3.0.0",
	}
	require.Equal(t, expected, a[0])
}
