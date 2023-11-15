package packagestructure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
)

var (
	manifestInvalidYAML = `"`
	manifestUnknownGK   = `apiVersion: banana/v3
kind: Bread`
	manifestUnknownVersion = `apiVersion: manifests.package-operator.run/v500
kind: PackageManifest`
)

func Test_manifestFromFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		yamlBytes []byte
		reason    packagetypes.ViolationReason
		details   string
	}{
		{
			name:      "invalid YAML",
			yamlBytes: []byte(manifestInvalidYAML),
			reason:    packagetypes.ViolationReasonInvalidYAML,
			details:   "error converting YAML to JSON: yaml: found unexpected end of stream",
		},
		{
			name:      "invalid GK",
			yamlBytes: []byte(manifestUnknownGK),
			reason:    packagetypes.ViolationReasonUnknownGVK,
			details:   "GroupKind must be PackageManifest.manifests.package-operator.run, is: Bread.banana",
		},
		{
			name:      "invalid version",
			yamlBytes: []byte(manifestUnknownVersion),
			reason:    packagetypes.ViolationReasonUnknownGVK,
			details:   "unknown version v500, supported versions: v1alpha1",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			path := "xxx.yml"
			m, err := manifestFromFile(ctx, scheme, path, test.yamlBytes)
			assert.Nil(t, m)
			var verr packagetypes.ViolationError
			if assert.ErrorAs(t, err, &verr) { //nolint:testifylint
				assert.Equal(t, test.reason, verr.Reason)
				assert.Equal(t, test.details, verr.Details)
				assert.Equal(t, path, verr.Path)
			}
		})
	}
}

var (
	manifestLockUnknownGK = `apiVersion: banana/v3
kind: Bread`
	manifestLockUnknownVersion = `apiVersion: manifests.package-operator.run/v500
kind: PackageManifestLock`
)

func Test_manifestLockFromFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		yamlBytes []byte
		reason    packagetypes.ViolationReason
		details   string
	}{
		{
			name:      "invalid YAML",
			yamlBytes: []byte(manifestInvalidYAML),
			reason:    packagetypes.ViolationReasonInvalidYAML,
			details:   "error converting YAML to JSON: yaml: found unexpected end of stream",
		},
		{
			name:      "invalid GK",
			yamlBytes: []byte(manifestLockUnknownGK),
			reason:    packagetypes.ViolationReasonUnknownGVK,
			details:   "GroupKind must be PackageManifestLock.manifests.package-operator.run, is: Bread.banana",
		},
		{
			name:      "invalid version",
			yamlBytes: []byte(manifestLockUnknownVersion),
			reason:    packagetypes.ViolationReasonUnknownGVK,
			details:   "unknown version v500, supported versions: v1alpha1",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			path := "xxx.yml"
			m, err := manifestLockFromFile(ctx, scheme, path, test.yamlBytes)
			assert.Nil(t, m)
			var verr packagetypes.ViolationError
			if assert.ErrorAs(t, err, &verr) { //nolint:testifylint
				assert.Equal(t, test.reason, verr.Reason)
				assert.Equal(t, test.details, verr.Details)
				assert.Equal(t, path, verr.Path)
			}
		})
	}
}

func TestToV1Alpha1ManifestLock(t *testing.T) {
	t.Parallel()
	internalLock := &manifests.PackageManifestLock{}
	lock, err := ToV1Alpha1ManifestLock(internalLock)
	require.NoError(t, err)
	assert.NotNil(t, lock)
}
