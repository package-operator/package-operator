package packagevalidation

import (
	"context"
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/stretchr/testify/require"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
)

var (
	errMan     = errors.New("manifest error")
	errManLock = errors.New("manifest lock error")
)

func TestPackageManifestValidator(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		ManifestErr     error
		HasManifestLock bool
		ManifestLockErr error
		ExpectedErr     error
	}{
		"no errors":             {},
		"no errors w lock":      {HasManifestLock: true},
		"manifest error":        {ManifestErr: errMan, HasManifestLock: true, ExpectedErr: errMan},
		"manifest error w lock": {ManifestErr: errMan, HasManifestLock: true, ExpectedErr: errMan},
		"manifest lock error":   {HasManifestLock: true, ManifestLockErr: errManLock, ExpectedErr: errManLock},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			valPkgManCount := 0
			valPkgManFn := func(context.Context, *manifests.PackageManifest) (field.ErrorList, error) {
				valPkgManCount++
				return field.ErrorList{}, tc.ManifestErr
			}
			valPkgManLockCount := 0
			valPkgManLockFn := func(context.Context, *manifests.PackageManifestLock) (field.ErrorList, error) {
				valPkgManLockCount++
				return field.ErrorList{}, tc.ManifestLockErr
			}
			manifestV := PackageManifestValidator{
				validatePackageManifest:     valPkgManFn,
				validatePackageManifestLock: valPkgManLockFn,
			}

			var pkgManLock *manifests.PackageManifestLock
			expectedValPkgManLockCount := 0
			if tc.HasManifestLock {
				pkgManLock = &manifests.PackageManifestLock{}
				if tc.ManifestErr == nil {
					expectedValPkgManLockCount = 1
				}
			}
			pkg := &packagetypes.Package{
				Manifest:     &manifests.PackageManifest{},
				ManifestLock: pkgManLock,
			}

			err := manifestV.ValidatePackage(context.Background(), pkg)
			require.Equal(t, tc.ExpectedErr, err)
			require.Equal(t, 1, valPkgManCount)
			require.Equal(t, expectedValPkgManLockCount, valPkgManLockCount)
		})
	}
}

func TestPackageScopeValidator(t *testing.T) {
	t.Parallel()

	scopeV := PackageScopeValidator(manifests.PackageManifestScopeCluster)

	ctx := context.Background()
	manifest := &manifests.PackageManifest{
		Spec: manifests.PackageManifestSpec{
			Scopes: []manifests.PackageManifestScope{manifests.PackageManifestScopeNamespaced},
		},
	}
	err := scopeV.ValidatePackage(ctx, &packagetypes.Package{
		Manifest: manifest,
	})
	require.EqualError(t, err, "Package unsupported scope in manifest.yaml")
}
