package updatecmd_test

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/stretchr/testify/require"

	"package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/cmd/kubectl-package/command/updatecmd"
	"package-operator.run/package-operator/internal/packages/packagecontent"
)

type UpdateError struct {
	Msg string
}

func (u UpdateError) Error() string {
	return u.Msg
}

const (
	loadPackageErrorMsg    = "load package error"
	retrieveDigestErrorMsg = "retrieve digest error"
	writeLockFileErrorMsg  = "write lock file error"
	testDigest             = "01234"
	testOtherDigest        = "56789"
)

var (
	errLoadPackage    = UpdateError{Msg: loadPackageErrorMsg}
	errRetrieveDigest = UpdateError{Msg: retrieveDigestErrorMsg}
	errWriteLockFile  = UpdateError{Msg: writeLockFileErrorMsg}
)

var testManifest = &v1alpha1.PackageManifest{Spec: v1alpha1.PackageManifestSpec{
	Images: []v1alpha1.PackageManifestImage{
		{Name: "nginx1", Image: "nginx:1.22.1"},
		{Name: "nginx2", Image: "nginx:1.23.3"},
	},
}}

var testPkgNoLockFile = packagecontent.Package{
	PackageManifest: testManifest,
}

var testPkgDifferentLockFile1 = packagecontent.Package{
	PackageManifest: testManifest,
	PackageManifestLock: &v1alpha1.PackageManifestLock{Spec: v1alpha1.PackageManifestLockSpec{
		Images: []v1alpha1.PackageManifestLockImage{
			{Name: "nginx1", Image: "nginx:1.22.1", Digest: testOtherDigest},
		},
	}},
}

var testPkgDifferentLockFile2 = packagecontent.Package{
	PackageManifest: testManifest,
	PackageManifestLock: &v1alpha1.PackageManifestLock{Spec: v1alpha1.PackageManifestLockSpec{
		Images: []v1alpha1.PackageManifestLockImage{
			{Name: "nginx1", Image: "nginx:1.22.1", Digest: testDigest},
			{Name: "foobar", Image: "foobar:2.18", Digest: testOtherDigest},
		},
	}},
}

var testPkgSameLockFile = packagecontent.Package{
	PackageManifest: testManifest,
	PackageManifestLock: &v1alpha1.PackageManifestLock{Spec: v1alpha1.PackageManifestLockSpec{
		Images: []v1alpha1.PackageManifestLockImage{
			{Name: "nginx1", Image: "nginx:1.22.1", Digest: testDigest},
			{Name: "nginx2", Image: "nginx:1.23.3", Digest: testDigest},
		},
	}},
}

func TestUpdateCommand(t *testing.T) {
	tests := []struct {
		name                string
		pkg                 *packagecontent.Package
		loadPackageError    bool
		retrieveDigestError bool
		retrieveDigestCount int
		writeLockFileError  bool
		writeLockFileCount  int
		expectError         string
	}{
		{
			name:             loadPackageErrorMsg,
			pkg:              &testPkgNoLockFile,
			loadPackageError: true,
			expectError:      loadPackageErrorMsg,
		},
		{
			name:                retrieveDigestErrorMsg,
			pkg:                 &testPkgNoLockFile,
			retrieveDigestError: true,
			retrieveDigestCount: 1,
			expectError:         retrieveDigestErrorMsg,
		},
		{
			name:                writeLockFileErrorMsg,
			pkg:                 &testPkgNoLockFile,
			retrieveDigestCount: 2,
			writeLockFileError:  true,
			writeLockFileCount:  1,
			expectError:         writeLockFileErrorMsg,
		},
		{
			name:                "all good with no lock file",
			pkg:                 &testPkgNoLockFile,
			retrieveDigestCount: 2,
			writeLockFileCount:  1,
		},
		{
			name:                "all good with different lock file 1",
			pkg:                 &testPkgDifferentLockFile1,
			retrieveDigestCount: 2,
			writeLockFileCount:  1,
		},
		{
			name:                "all good with different lock file 2",
			pkg:                 &testPkgDifferentLockFile2,
			retrieveDigestCount: 2,
			writeLockFileCount:  1,
		},
		{
			name:                "all good with same lock file",
			pkg:                 &testPkgSameLockFile,
			retrieveDigestCount: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var actualLoadPackageCount, actualRetrieveDigestCount, actualWriteLockFileCount int
			update := updatecmd.Update{
				LoadPackage: func(ctx context.Context, target string) (*packagecontent.Package, error) {
					actualLoadPackageCount++
					if test.loadPackageError {
						return nil, errLoadPackage
					}
					return test.pkg, nil
				},
				RetrieveDigest: func(ref string, opt ...crane.Option) (string, error) {
					actualRetrieveDigestCount++
					if test.retrieveDigestError {
						return "", errRetrieveDigest
					}
					return testDigest, nil
				},
				WriteLockFile: func(path string, data []byte) error {
					actualWriteLockFileCount++
					if test.writeLockFileError {
						return errWriteLockFile
					}
					return nil
				},
				Target: "testdata",
			}

			err := update.Run(context.Background())

			if test.expectError != "" {
				require.ErrorContains(t, err, test.expectError)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, 1, actualLoadPackageCount)
			require.Equal(t, test.retrieveDigestCount, actualRetrieveDigestCount)
			require.Equal(t, test.writeLockFileCount, actualWriteLockFileCount)
		})
	}
}
