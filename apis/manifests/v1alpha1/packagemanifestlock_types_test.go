package v1alpha1

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func TestPackageManifestLock_Validate(t *testing.T) {
	tests := []struct {
		name        string
		manifest    *PackageManifestLock
		errorString string // main error string that we are interested in, might return more.
	}{
		{
			name: "empty image",
			manifest: &PackageManifestLock{
				Spec: PackageManifestLockSpec{
					Images: []PackageManifestLockImage{{}},
				},
			},
			errorString: "spec.images[0].name: Invalid value: \"\": must be non empty",
		},
		{
			name: "empty image name",
			manifest: &PackageManifestLock{
				Spec: PackageManifestLockSpec{
					Images: []PackageManifestLockImage{{Image: "nginx:1.23.3", Digest: "123"}},
				},
			},
			errorString: "spec.images[0].name: Invalid value: \"\": must be non empty",
		},
		{
			name: "empty image identifier",
			manifest: &PackageManifestLock{
				Spec: PackageManifestLockSpec{
					Images: []PackageManifestLockImage{{Name: "nginx", Digest: "123"}},
				},
			},
			errorString: "spec.images[0].image: Invalid value: \"\": must be non empty",
		},
		{
			name: "empty image digest",
			manifest: &PackageManifestLock{
				Spec: PackageManifestLockSpec{
					Images: []PackageManifestLockImage{{Name: "nginx", Image: "nginx:1.23.3"}},
				},
			},
			errorString: "spec.images[0].digest: Invalid value: \"\": must be non empty",
		},
		{
			name: "duplicated image name",
			manifest: &PackageManifestLock{
				Spec: PackageManifestLockSpec{
					Images: []PackageManifestLockImage{
						{Name: "nginx", Image: "nginx:1.23.3", Digest: "123"},
						{Name: "nginx", Image: "nginx:1.22.1", Digest: "456"},
					},
				},
			},
			errorString: "spec.images[1].name: Invalid value: \"nginx\": must be unique",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.manifest.Validate()
			var aggregateErr utilerrors.Aggregate
			require.True(t, errors.As(err, &aggregateErr))

			var errorStrings []string
			for _, err := range aggregateErr.Errors() {
				errorStrings = append(errorStrings, err.Error())
			}

			assert.Contains(t, errorStrings, test.errorString)
		})
	}
}
