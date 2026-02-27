package packagevalidation

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
)

func TestLockfileDigestLookupValidator(t *testing.T) {
	t.Parallel()
	const (
		testDigest = "01234"
	)

	m := &digestResolverMock{}
	m.On("Digest", "index.docker.io/library/nginx@01234", mock.Anything).
		Return(testDigest, nil)

	ctx := context.Background()
	v := &LockfileDigestLookupValidator{
		digestLookupFn: m.Digest,
	}
	pkg := pkg(
		map[string]string{"nginx": "nginx:1.22.1"},
		map[string]LockImageTestData{"nginx": {Image: "nginx:1.22.1", Digest: testDigest}},
	)
	err := v.ValidatePackage(ctx, pkg)
	require.NoError(t, err)
}

func TestLockfileConsistencyValidator(t *testing.T) {
	t.Parallel()
	const (
		testDigest      = "01234"
		testOtherDigest = "56789"
	)

	tests := map[string]struct {
		name          string
		pkg           *packagetypes.Package
		expectedError string
	}{
		"no images/no lock file": {
			pkg: pkg(map[string]string{}, map[string]LockImageTestData{}),
		},
		"images/no lock file": {
			pkg: pkg(
				map[string]string{"nginx": "nginx:1.22.1"},
				map[string]LockImageTestData{}),
			expectedError: "Missing image in manifest.lock.yaml, but using PackageManifest.spec.images. Try running: kubectl package update", //nolint: lll
		},
		"images/lock file present with missing images": {
			pkg: pkg(
				map[string]string{"nginx": "nginx:1.22.1"},
				map[string]LockImageTestData{"foobar": {Image: "foobar:1.2.3", Digest: testDigest}},
			),
			expectedError: `Missing image in manifest.lock.yaml, but using PackageManifest.spec.images. Try running: kubectl package update: nginx`, //nolint: lll
		},
		"images/lock file present with extra images": {
			pkg: pkg(
				map[string]string{"nginx": "nginx:1.22.1"},
				map[string]LockImageTestData{
					"nginx":  {Image: "nginx:1.22.1", Digest: testDigest},
					"foobar": {Image: "foobar:1.2.3", Digest: testOtherDigest},
				},
			),
			expectedError: `Image specified in manifest does not match with lockfile. Try running: kubectl package update: foobar`, //nolint: lll
		},
		"images/lock file present with conflicting tags": {
			pkg: pkg(
				map[string]string{"nginx": "nginx:1.22.1"},
				map[string]LockImageTestData{"nginx": {Image: "nginx:1.23.3", Digest: testDigest}},
			),
			expectedError: `Image specified in manifest does not match with lockfile. Try running: kubectl package update: "nginx": "nginx:1.22.1" vs "nginx:1.23.3"`, //nolint: lll
		},
		"happy path": {
			pkg: pkg(
				map[string]string{"nginx": "nginx:1.22.1"},
				map[string]LockImageTestData{"nginx": {Image: "nginx:1.22.1", Digest: testDigest}},
			),
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			v := &LockfileConsistencyValidator{}
			err := v.ValidatePackage(ctx, tc.pkg)
			if tc.expectedError != "" {
				require.ErrorContains(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type LockImageTestData struct {
	Image  string
	Digest string
}

func pkg(manifestImages map[string]string, lockImages map[string]LockImageTestData) *packagetypes.Package {
	imgManifest := make([]manifests.PackageManifestImage, 0, len(manifestImages))
	for key, value := range manifestImages {
		imgManifest = append(imgManifest, manifests.PackageManifestImage{
			Name:  key,
			Image: value,
		})
	}

	var lock *manifests.PackageManifestLock
	if len(lockImages) > 0 {
		imgLock := make([]manifests.PackageManifestLockImage, 0, len(lockImages))
		for key, value := range lockImages {
			imgLock = append(imgLock, manifests.PackageManifestLockImage{
				Name:   key,
				Image:  value.Image,
				Digest: value.Digest,
			})
		}
		lock = &manifests.PackageManifestLock{
			Spec: manifests.PackageManifestLockSpec{
				Images: imgLock,
			},
		}
	}

	return &packagetypes.Package{
		Manifest: &manifests.PackageManifest{
			Spec: manifests.PackageManifestSpec{
				Images: imgManifest,
			},
		},
		ManifestLock: lock,
	}
}

type digestResolverMock struct {
	mock.Mock
}

func (m *digestResolverMock) Digest(ref string, opt ...crane.Option) (string, error) {
	args := m.Called(ref, opt)
	return args.String(0), args.Error(1)
}
