package packagemanifestvalidation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"package-operator.run/internal/apis/manifests"
)

func TestValidRepositoryEntryValidation(t *testing.T) {
	t.Parallel()

	entry := manifests.RepositoryEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar"},
		Data: manifests.RepositoryEntryData{
			Image:    "foo-img",
			Digest:   "bar-digest",
			Versions: []string{"1.0.0"},
			Constraints: []manifests.PackageManifestConstraint{
				{
					PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{
						Name:  "foo-vrs",
						Range: "1.2.3-1.3.0",
					},
				},
			},
			Name: "bar-entry",
		},
	}

	ctx := context.Background()

	errorList, err := ValidateRepositoryEntry(ctx, &entry)
	require.NoError(t, err)
	assert.Empty(t, errorList)
}

func TestBrokenRepositoryEntryValidation(t *testing.T) {
	t.Parallel()

	entry := manifests.RepositoryEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "", Namespace: ""},
		Data: manifests.RepositoryEntryData{
			Image:    "",
			Digest:   "",
			Versions: []string{},
			Constraints: []manifests.PackageManifestConstraint{
				{
					PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{
						Name:  "",
						Range: "",
					},
					Platform:      nil,
					UniqueInScope: nil,
				},
			},
			Name: "",
		},
	}

	ctx := context.Background()

	errorList, err := ValidateRepositoryEntry(ctx, &entry)
	require.NoError(t, err)
	assert.Len(t, errorList, 7)
}
