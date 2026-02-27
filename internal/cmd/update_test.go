package cmd

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages"
)

func TestUpdate(t *testing.T) {
	t.Parallel()

	type expected struct {
		LockData string
		Error    error
	}

	now := metav1.Now()

	for name, tc := range map[string]struct {
		Package       *packages.Package
		ImageToDigest map[string]string
		Expected      expected
	}{
		"no existing lock file": {
			Package: &testPkgNoLockFile,
			ImageToDigest: map[string]string{
				"nginx:1.22.1": "01234",
				"nginx:1.23.3": "56789",
			},
			Expected: expected{
				LockData: strings.Join([]string{
					"apiVersion: manifests.package-operator.run/v1alpha1",
					"kind: PackageManifestLock",
					"metadata:",
					fmt.Sprintf("  creationTimestamp: %q", now.UTC().Format(time.RFC3339)),
					"spec:",
					"  images:",
					`  - digest: "01234"`,
					"    image: nginx:1.22.1",
					"    name: nginx1",
					`  - digest: "56789"`,
					"    image: nginx:1.23.3",
					"    name: nginx2",
					"",
				}, "\n"),
			},
		},
		"lock file exists/conflicting digests": {
			Package: &testPkgDifferentLockFile1,
			ImageToDigest: map[string]string{
				"nginx:1.22.1": "01234",
				"nginx:1.23.3": "56789",
			},
			Expected: expected{
				LockData: strings.Join([]string{
					"apiVersion: manifests.package-operator.run/v1alpha1",
					"kind: PackageManifestLock",
					"metadata:",
					fmt.Sprintf("  creationTimestamp: %q", now.UTC().Format(time.RFC3339)),
					"spec:",
					"  images:",
					`  - digest: "01234"`,
					"    image: nginx:1.22.1",
					"    name: nginx1",
					`  - digest: "56789"`,
					"    image: nginx:1.23.3",
					"    name: nginx2",
					"",
				}, "\n"),
			},
		},
		"lock file exists/conflicting image references": {
			Package: &testPkgDifferentLockFile2,
			ImageToDigest: map[string]string{
				"nginx:1.22.1": "01234",
				"nginx:1.23.3": "56789",
			},
			Expected: expected{
				LockData: strings.Join([]string{
					"apiVersion: manifests.package-operator.run/v1alpha1",
					"kind: PackageManifestLock",
					"metadata:",
					fmt.Sprintf("  creationTimestamp: %q", now.UTC().Format(time.RFC3339)),
					"spec:",
					"  images:",
					`  - digest: "01234"`,
					"    image: nginx:1.22.1",
					"    name: nginx1",
					`  - digest: "56789"`,
					"    image: nginx:1.23.3",
					"    name: nginx2",
					"",
				}, "\n"),
			},
		},
		"lock file exists/no changes": {
			Package: &testPkgSameLockFile,
			ImageToDigest: map[string]string{
				"nginx:1.22.1": "01234",
				"nginx:1.23.3": "01234",
			},
			Expected: expected{
				Error: ErrLockDataUnchanged,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mLoader := &packageLoaderMock{}
			mLoader.
				On("LoadPackage", mock.Anything, "src").
				Return(tc.Package, nil)

			mResolver := &digestResolverMock{}

			for ref, digest := range tc.ImageToDigest {
				mResolver.
					On("ResolveDigest", ref, mock.Anything).
					Return(digest, nil)
			}

			mClock := &clockMock{}
			mClock.
				On("Now").
				Return(now)

			update := NewUpdate(
				WithClock{Clock: mClock},
				WithPackageLoader{Loader: mLoader},
				WithDigestResolver{Resolver: mResolver},
			)

			data, err := update.GenerateLockData(context.Background(), "src")
			require.ErrorIs(t, err, tc.Expected.Error)

			assert.Equal(t, tc.Expected.LockData, string(data))
		})
	}
}

type digestResolverMock struct {
	mock.Mock
}

func (m *digestResolverMock) ResolveDigest(ref string, opts ...ResolveDigestOption) (string, error) {
	actualArgs := make([]any, 0, 1+len(opts))
	actualArgs = append(actualArgs, ref)

	for _, opt := range opts {
		actualArgs = append(actualArgs, opt)
	}

	args := m.Called(actualArgs...)

	return args.String(0), args.Error(1)
}

type packageLoaderMock struct {
	mock.Mock
}

func (m *packageLoaderMock) LoadPackage(ctx context.Context, path string) (*packages.Package, error) {
	args := m.Called(ctx, path)
	pkg, _ := args.Get(0).(*packages.Package)
	return pkg, args.Error(1)
}

type clockMock struct {
	mock.Mock
}

func (m *clockMock) Now() metav1.Time {
	args := m.Called()

	return args.Get(0).(metav1.Time)
}

const (
	testDigest      = "01234"
	testOtherDigest = "56789"
)

var testPkgNoLockFile = packages.Package{
	Manifest: testManifest,
}

var testPkgDifferentLockFile1 = packages.Package{
	Manifest: testManifest,
	ManifestLock: &manifests.PackageManifestLock{Spec: manifests.PackageManifestLockSpec{
		Images: []manifests.PackageManifestLockImage{
			{Name: "nginx1", Image: "nginx:1.22.1", Digest: testOtherDigest},
		},
	}},
}

var testPkgDifferentLockFile2 = packages.Package{
	Manifest: testManifest,
	ManifestLock: &manifests.PackageManifestLock{Spec: manifests.PackageManifestLockSpec{
		Images: []manifests.PackageManifestLockImage{
			{Name: "nginx1", Image: "nginx:1.22.1", Digest: testDigest},
			{Name: "foobar", Image: "foobar:2.18", Digest: testOtherDigest},
		},
	}},
}

var testPkgSameLockFile = packages.Package{
	Manifest: testManifest,
	ManifestLock: &manifests.PackageManifestLock{Spec: manifests.PackageManifestLockSpec{
		Images: []manifests.PackageManifestLockImage{
			{Name: "nginx1", Image: "nginx:1.22.1", Digest: testDigest},
			{Name: "nginx2", Image: "nginx:1.23.3", Digest: testDigest},
		},
	}},
}

var testManifest = &manifests.PackageManifest{Spec: manifests.PackageManifestSpec{
	Images: []manifests.PackageManifestImage{
		{Name: "nginx1", Image: "nginx:1.22.1"},
		{Name: "nginx2", Image: "nginx:1.23.3"},
	},
}}
