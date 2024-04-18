package packagevalidation

import (
	"context"
	"errors"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

type PackageScopeValidatorTestCase struct {
	rootScopes      []manifests.PackageManifestScope
	componentScopes []manifests.PackageManifestScope
	desiredScope    manifests.PackageManifestScope
	expectError     bool
}

func TestPackageScopeValidator(t *testing.T) {
	t.Parallel()

	possibleRootScopes := [][]manifests.PackageManifestScope{
		{manifests.PackageManifestScopeNamespaced},
		{manifests.PackageManifestScopeCluster},
		{manifests.PackageManifestScopeNamespaced, manifests.PackageManifestScopeCluster},
	}

	possibleComponentScopes := [][]manifests.PackageManifestScope{
		nil,
		{manifests.PackageManifestScopeNamespaced},
		{manifests.PackageManifestScopeCluster},
		{manifests.PackageManifestScopeNamespaced, manifests.PackageManifestScopeCluster},
	}

	possibleDesiredScopes := []manifests.PackageManifestScope{
		manifests.PackageManifestScopeNamespaced, manifests.PackageManifestScopeCluster,
	}

	testCases := map[string]PackageScopeValidatorTestCase{}

	for i, rootScopes := range possibleRootScopes {
		for j, componentScopes := range possibleComponentScopes {
			for k, desiredScope := range possibleDesiredScopes {
				expectError := true
				for _, rootScope := range rootScopes {
					if rootScope == desiredScope {
						expectError = false
					}
				}
				// use indexes of each array to build test case name
				testCases[fmt.Sprintf("R:%d_C:%d_D:%d_E:%t", i, j, k, expectError)] = PackageScopeValidatorTestCase{
					rootScopes, componentScopes, desiredScope, expectError,
				}
			}
		}
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			var components []packagetypes.Package
			if tc.componentScopes != nil {
				components = []packagetypes.Package{
					{
						Manifest: &manifests.PackageManifest{
							ObjectMeta: metav1.ObjectMeta{Name: "test"},
							Spec: manifests.PackageManifestSpec{
								Scopes: tc.componentScopes,
							},
						},
					},
				}
			}

			pkg := &packagetypes.Package{
				Manifest: &manifests.PackageManifest{
					ObjectMeta: metav1.ObjectMeta{Name: "root"},
					Spec: manifests.PackageManifestSpec{
						Scopes: tc.rootScopes,
					},
				},
				Components: components,
			}

			scopeV := PackageScopeValidator(tc.desiredScope)

			err := scopeV.ValidatePackage(ctx, pkg)
			if tc.expectError {
				require.EqualError(t, err, "Package unsupported scope in manifest.yaml")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
