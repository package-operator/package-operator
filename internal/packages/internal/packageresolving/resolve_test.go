package packageresolving_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages"
	"package-operator.run/internal/packages/internal/packageresolving"
)

func TestResolveNothing(t *testing.T) {
	t.Parallel()
	idx := packages.NewMultiRepositoryIndex()
	r := packageresolving.BuildResolver{Loader: func(ctx context.Context, pmr []manifests.PackageManifestRepository) (*packages.MultiRepositoryIndex, error) {
		return idx, nil
	}}
	err := r.Solve()
	require.NoError(t, err)
}

func TestResolveBasic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pkg := &manifests.PackageManifest{
		ObjectMeta: metav1.ObjectMeta{Name: "mainpkg"},
		Spec: manifests.PackageManifestSpec{
			Dependencies: []manifests.PackageManifestDependency{
				{Image: &manifests.PackageManifestDependencyImage{Name: "depname", Package: "deppkg.deprepo"}},
			},
		},
	}

	idx := packages.NewMultiRepositoryIndex()

	require.NoError(t, idx.Add(ctx, packages.Entry{
		RepositoryName: "deprepo",
		RepositoryEntry: &manifests.RepositoryEntry{
			Data: manifests.RepositoryEntryData{Image: "image", Digest: "digest", Name: "deppkg", Versions: []string{"1.0.0", "2.0.0", "3.0.0"}},
		},
	}))

	r := packageresolving.BuildResolver{Loader: func(ctx context.Context, pmr []manifests.PackageManifestRepository) (*packages.MultiRepositoryIndex, error) {
		return idx, nil
	}}
	lcks, err := r.AddManifest(ctx, pkg)
	require.NoError(t, err)
	require.NoError(t, r.Solve())
	require.Len(t, *lcks, 1)
	expected := manifests.PackageManifestLockDependency{Name: "depname", Image: "image", Digest: "digest", Version: "3.0.0"}
	require.Equal(t, expected, (*lcks)[0])
}

func TestResolvePlatformGood(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pkg := &manifests.PackageManifest{
		ObjectMeta: metav1.ObjectMeta{Name: "mainpkg"},
		Spec: manifests.PackageManifestSpec{
			Constraints: []manifests.PackageManifestConstraint{
				{PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{Name: "platform", Range: ">=4.12.x"}},
			},
			Dependencies: []manifests.PackageManifestDependency{
				{Image: &manifests.PackageManifestDependencyImage{Name: "depname", Package: "deppkg.deprepo"}},
			},
		},
	}

	idx := packages.NewMultiRepositoryIndex()

	require.NoError(t, idx.Add(ctx, packages.Entry{
		RepositoryName: "deprepo",
		RepositoryEntry: &manifests.RepositoryEntry{
			Data: manifests.RepositoryEntryData{
				Image:    "image",
				Digest:   "digest",
				Name:     "deppkg",
				Versions: []string{"3.0.0", "2.0.0", "1.0.0"},
				Constraints: []manifests.PackageManifestConstraint{{
					Platform:        []manifests.PlatformName{"platform", "formplat"},
					PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{Name: "platform", Range: ">=4.11.x"},
				}},
			},
		},
	}))

	r := packageresolving.BuildResolver{Loader: func(ctx context.Context, pmr []manifests.PackageManifestRepository) (*packages.MultiRepositoryIndex, error) {
		return idx, nil
	}}
	lcks, err := r.AddManifest(ctx, pkg)
	require.NoError(t, err)
	require.NoError(t, r.Solve())
	require.Len(t, *lcks, 1)
	expected := manifests.PackageManifestLockDependency{Name: "depname", Image: "image", Digest: "digest", Version: "3.0.0"}
	require.Equal(t, expected, (*lcks)[0])
}

func TestResolvePlatformBad(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pkg := &manifests.PackageManifest{
		ObjectMeta: metav1.ObjectMeta{Name: "mainpkg"},
		Spec: manifests.PackageManifestSpec{
			Constraints: []manifests.PackageManifestConstraint{
				{PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{Name: "platform", Range: ">=4.11.x"}},
			},
			Repositories: []manifests.PackageManifestRepository{},
			Dependencies: []manifests.PackageManifestDependency{
				{Image: &manifests.PackageManifestDependencyImage{Name: "depname", Package: "deppkg.deprepo"}},
			},
		},
	}

	idx := packages.NewMultiRepositoryIndex()

	require.NoError(t, idx.Add(ctx, packages.Entry{
		RepositoryName: "deprepo",
		RepositoryEntry: &manifests.RepositoryEntry{
			Data: manifests.RepositoryEntryData{
				Image:    "image",
				Digest:   "digest",
				Name:     "deppkg",
				Versions: []string{"3.0.0", "2.0.0", "1.0.0"},
				Constraints: []manifests.PackageManifestConstraint{{
					PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{Name: "platform", Range: ">=4.12.x"},
				}},
			},
		},
	}))

	r := packageresolving.BuildResolver{Loader: func(ctx context.Context, pmr []manifests.PackageManifestRepository) (*packages.MultiRepositoryIndex, error) {
		return idx, nil
	}}
	_, err := r.AddManifest(ctx, pkg)
	require.NoError(t, err)
	require.Error(t, r.Solve())
}

func TestResolvePlatformBadVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pkg := &manifests.PackageManifest{
		ObjectMeta: metav1.ObjectMeta{Name: "mainpkg"},
		Spec: manifests.PackageManifestSpec{
			Constraints: []manifests.PackageManifestConstraint{
				{PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{Name: "platform", Range: ">=4.11.x"}},
			},
			Dependencies: []manifests.PackageManifestDependency{
				{Image: &manifests.PackageManifestDependencyImage{Name: "depname", Package: "deppkg.deprepo", Range: ">=4"}},
			},
		},
	}

	idx := packages.NewMultiRepositoryIndex()

	require.NoError(t, idx.Add(ctx, packages.Entry{
		RepositoryName: "deprepo",
		RepositoryEntry: &manifests.RepositoryEntry{
			Data: manifests.RepositoryEntryData{
				Image:    "image",
				Digest:   "digest",
				Name:     "deppkg",
				Versions: []string{"3.0.0", "2.0.0", "1.0.0"},
				Constraints: []manifests.PackageManifestConstraint{
					{PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{Name: "platform", Range: ">=4.12.x"}},
				},
			},
		},
	}))

	r := packageresolving.BuildResolver{Loader: func(ctx context.Context, pmr []manifests.PackageManifestRepository) (*packages.MultiRepositoryIndex, error) {
		return idx, nil
	}}
	_, err := r.AddManifest(ctx, pkg)
	require.NoError(t, err)
	require.Error(t, r.Solve())
}

func TestResolvePlatformBrokenVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pkg := &manifests.PackageManifest{
		ObjectMeta: metav1.ObjectMeta{Name: "mainpkg"},
		Spec: manifests.PackageManifestSpec{
			Constraints: []manifests.PackageManifestConstraint{
				{PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{Name: "platform", Range: ">=4.11.x"}},
			},
			Dependencies: []manifests.PackageManifestDependency{
				{Image: &manifests.PackageManifestDependencyImage{Name: "depname", Package: "deppkg.deprepo", Range: ">=asdaa"}},
			},
		},
	}

	idx := packages.NewMultiRepositoryIndex()

	require.NoError(t, idx.Add(ctx, packages.Entry{
		RepositoryName: "deprepo",
		RepositoryEntry: &manifests.RepositoryEntry{
			Data: manifests.RepositoryEntryData{
				Image:    "image",
				Digest:   "digest",
				Name:     "deppkg",
				Versions: []string{"3.0.0", "2.0.0", "1.0.0"},
				Constraints: []manifests.PackageManifestConstraint{
					{PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{Name: "platform", Range: ">=4.12.x"}},
				},
			},
		},
	}))

	r := packageresolving.BuildResolver{Loader: func(ctx context.Context, pmr []manifests.PackageManifestRepository) (*packages.MultiRepositoryIndex, error) {
		return idx, nil
	}}
	_, err := r.AddManifest(ctx, pkg)
	require.Error(t, err)
	require.NoError(t, r.Solve())
}
