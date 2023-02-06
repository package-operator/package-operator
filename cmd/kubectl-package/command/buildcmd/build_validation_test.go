package buildcmd

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/stretchr/testify/require"

	"package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages/packagecontent"
)

type buildValidationTestError struct {
	Msg string
}

func (u buildValidationTestError) Error() string {
	return u.Msg
}

const (
	retrieveDigestErrorMsg = "retrieve digest error"
	testDigest             = "01234"
	testOtherDigest        = "56789"
)

var errRetrieveDigest = buildValidationTestError{Msg: retrieveDigestErrorMsg}

func TestPreBuildValidation(t *testing.T) {
	tests := []struct {
		name                string
		pkg                 *packagecontent.Package
		retrieveDigestError bool
		retrieveDigestCount int
		expectError         string
	}{
		{
			name: "no images and no lock file is ok",
			pkg:  pkg(map[string]string{}, map[string]LockImageTestData{}),
		},
		{
			name:        "some images and no lock file error",
			pkg:         pkg(map[string]string{"nginx": "nginx:1.22.1"}, map[string]LockImageTestData{}),
			expectError: "manifest.lock.yaml is missing",
		},
		{
			name: "missing images in lock file error",
			pkg: pkg(
				map[string]string{"nginx": "nginx:1.22.1"},
				map[string]LockImageTestData{"foobar": {Image: "foobar:1.2.3", Digest: testDigest}},
			),
			expectError: "image \"nginx\" exists in manifest but not in lock file",
		},
		{
			name: "extra image in lock file error",
			pkg: pkg(
				map[string]string{"nginx": "nginx:1.22.1"},
				map[string]LockImageTestData{
					"nginx":  {Image: "nginx:1.22.1", Digest: testDigest},
					"foobar": {Image: "foobar:1.2.3", Digest: testOtherDigest},
				},
			),
			expectError: "image \"foobar\" exists in lock but not in manifest file",
		},
		{
			name: "different image id error",
			pkg: pkg(
				map[string]string{"nginx": "nginx:1.22.1"},
				map[string]LockImageTestData{"nginx": {Image: "nginx:1.23.3", Digest: testDigest}},
			),
			expectError: "tags for image \"nginx\" differ between manifest and lock file: \"nginx:1.22.1\" vs \"nginx:1.23.3\"",
		},
		{
			name: "retrieve digest error",
			pkg: pkg(
				map[string]string{"nginx": "nginx:1.22.1"},
				map[string]LockImageTestData{"nginx": {Image: "nginx:1.22.1", Digest: testDigest}},
			),
			retrieveDigestError: true,
			retrieveDigestCount: 1,
			expectError:         "image \"nginx\" digest error",
		},
		{
			name: "all good",
			pkg: pkg(
				map[string]string{"nginx": "nginx:1.22.1"},
				map[string]LockImageTestData{"nginx": {Image: "nginx:1.22.1", Digest: testDigest}},
			),
			retrieveDigestCount: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var actualRetrieveDigestCount int
			err := preBuildValidation(test.pkg, func(ref string, opt ...crane.Option) (string, error) {
				actualRetrieveDigestCount++
				if test.retrieveDigestError {
					return "", errRetrieveDigest
				}
				return testDigest, nil
			})

			if test.expectError != "" {
				require.ErrorContains(t, err, test.expectError)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, test.retrieveDigestCount, actualRetrieveDigestCount)
		})
	}
}

type LockImageTestData struct {
	Image  string
	Digest string
}

func pkg(manifestImages map[string]string, lockImages map[string]LockImageTestData) *packagecontent.Package {
	var imgManifest []v1alpha1.PackageManifestImage
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
