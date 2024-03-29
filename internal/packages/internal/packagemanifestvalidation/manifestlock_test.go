package packagemanifestvalidation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/apis/manifests"
)

func TestValidatePackageManifestLock(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		manifest       *manifests.PackageManifestLock
		expectedErrors []string // main error string that we are interested in, might return more.
	}{
		{
			name: "empty image",
			manifest: &manifests.PackageManifestLock{
				Spec: manifests.PackageManifestLockSpec{
					Images: []manifests.PackageManifestLockImage{{}},
				},
			},
			expectedErrors: []string{
				"spec.images[0].name: Invalid value: \"\": must be non empty",
				"spec.images[0].image: Invalid value: \"\": must be non empty",
				"spec.images[0].digest: Invalid value: \"\": must be non empty",
			},
		},
		{
			name: "empty image name",
			manifest: &manifests.PackageManifestLock{
				Spec: manifests.PackageManifestLockSpec{
					Images: []manifests.PackageManifestLockImage{{Image: "nginx:1.23.3", Digest: "123"}},
				},
			},
			expectedErrors: []string{"spec.images[0].name: Invalid value: \"\": must be non empty"},
		},
		{
			name: "empty image identifier",
			manifest: &manifests.PackageManifestLock{
				Spec: manifests.PackageManifestLockSpec{
					Images: []manifests.PackageManifestLockImage{{Name: "nginx", Digest: "123"}},
				},
			},
			expectedErrors: []string{"spec.images[0].image: Invalid value: \"\": must be non empty"},
		},
		{
			name: "empty image digest",
			manifest: &manifests.PackageManifestLock{
				Spec: manifests.PackageManifestLockSpec{
					Images: []manifests.PackageManifestLockImage{{Name: "nginx", Image: "nginx:1.23.3"}},
				},
			},
			expectedErrors: []string{"spec.images[0].digest: Invalid value: \"\": must be non empty"},
		},
		{
			name: "duplicated image name",
			manifest: &manifests.PackageManifestLock{
				Spec: manifests.PackageManifestLockSpec{
					Images: []manifests.PackageManifestLockImage{
						{Name: "nginx", Image: "nginx:1.23.3", Digest: "123"},
						{Name: "nginx", Image: "nginx:1.22.1", Digest: "456"},
					},
				},
			},
			expectedErrors: []string{"spec.images[1].name: Invalid value: \"nginx\": must be unique"},
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			ferrs, err := ValidatePackageManifestLock(ctx, test.manifest)
			require.NoError(t, err)
			require.Len(t, ferrs, len(test.expectedErrors))

			var errorStrings []string
			for _, err := range ferrs {
				errorStrings = append(errorStrings, err.Error())
			}
			for _, expectedError := range test.expectedErrors {
				assert.Contains(t, errorStrings, expectedError)
			}
		})
	}
}
