package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages/packagecontent"
)

func TestPreBuildValidation(t *testing.T) {
	t.Parallel()

	const (
		testDigest      = "01234"
		testOtherDigest = "56789"
	)

	for name, tc := range map[string]struct {
		Package             *packagecontent.Package
		RetrieveDigestError bool
		ImageToDigest       map[string]string
		ExpectError         string
	}{
		"no images/no lock file": {
			Package: pkg(map[string]string{}, map[string]LockImageTestData{}),
		},
		"images/no lock file": {
			Package:     pkg(map[string]string{"nginx": "nginx:1.22.1"}, map[string]LockImageTestData{}),
			ExpectError: "manifest.lock.yaml is missing",
		},
		"images/lock file present with missing images": {
			Package: pkg(
				map[string]string{"nginx": "nginx:1.22.1"},
				map[string]LockImageTestData{"foobar": {Image: "foobar:1.2.3", Digest: testDigest}},
			),
			ExpectError: `image "nginx" exists in manifest but not in lock file`,
		},
		"images/lock file present with extra images": {
			Package: pkg(
				map[string]string{"nginx": "nginx:1.22.1"},
				map[string]LockImageTestData{
					"nginx":  {Image: "nginx:1.22.1", Digest: testDigest},
					"foobar": {Image: "foobar:1.2.3", Digest: testOtherDigest},
				},
			),
			ExpectError: `image "foobar" exists in lock but not in manifest file`,
		},
		"images/lock file present with conflicting tags": {
			Package: pkg(
				map[string]string{"nginx": "nginx:1.22.1"},
				map[string]LockImageTestData{"nginx": {Image: "nginx:1.23.3", Digest: testDigest}},
			),
			ExpectError: `tags for image "nginx" differ between manifest and lock file: "nginx:1.22.1" vs "nginx:1.23.3"`,
		},
		"happy path": {
			Package: pkg(
				map[string]string{"nginx": "nginx:1.22.1"},
				map[string]LockImageTestData{"nginx": {Image: "nginx:1.22.1", Digest: testDigest}},
			),
			ImageToDigest: map[string]string{
				"index.docker.io/library/nginx@01234": testDigest,
			},
		},
	} {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mResolver := &digestResolverMock{}

			for ref, digest := range tc.ImageToDigest {
				mResolver.
					On("ResolveDigest", ref).
					Return(digest, nil)
			}

			scheme, err := NewScheme()
			require.NoError(t, err)

			build := NewBuild(
				scheme,
				WithDigestResolver{Resolver: mResolver},
			)

			err = build.validatePackage(tc.Package)

			if tc.ExpectError != "" {
				require.ErrorContains(t, err, tc.ExpectError)
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

func pkg(manifestImages map[string]string, lockImages map[string]LockImageTestData) *packagecontent.Package {
	imgManifest := make([]v1alpha1.PackageManifestImage, 0, len(manifestImages))
	for key, value := range manifestImages {
		imgManifest = append(imgManifest, v1alpha1.PackageManifestImage{Name: key, Image: value})
	}

	var lock *v1alpha1.PackageManifestLock
	if len(lockImages) > 0 {
		var imgLock []v1alpha1.PackageManifestLockImage
		for key, value := range lockImages {
			imgLock = append(imgLock, v1alpha1.PackageManifestLockImage{Name: key, Image: value.Image, Digest: value.Digest})
		}
		lock = &v1alpha1.PackageManifestLock{Spec: v1alpha1.PackageManifestLockSpec{Images: imgLock}}
	}

	return &packagecontent.Package{
		PackageManifest:     &v1alpha1.PackageManifest{Spec: v1alpha1.PackageManifestSpec{Images: imgManifest}},
		PackageManifestLock: lock,
	}
}
