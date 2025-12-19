package manifests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRepositoryEntry_Structure(t *testing.T) {
	t.Parallel()

	entry := &RepositoryEntry{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "manifests.package-operator.run/v1alpha1",
			Kind:       "RepositoryEntry",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-entry",
			Namespace: "test-namespace",
		},
		Data: RepositoryEntryData{
			Image:    "quay.io/example/test-package",
			Digest:   "sha256:abcd1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab",
			Versions: []string{"1.0.0", "1.1.0", "2.0.0"},
			Name:     "test-package",
			Constraints: []PackageManifestConstraint{
				{
					Platform: []PlatformName{Kubernetes, OpenShift},
					PlatformVersion: &PackageManifestPlatformVersionConstraint{
						Name:  Kubernetes,
						Range: ">=1.20",
					},
				},
			},
		},
	}

	assert.Equal(t, "manifests.package-operator.run/v1alpha1", entry.APIVersion)
	assert.Equal(t, "RepositoryEntry", entry.Kind)
	assert.Equal(t, "test-entry", entry.Name)
	assert.Equal(t, "test-namespace", entry.Namespace)
	assert.Equal(t, "quay.io/example/test-package", entry.Data.Image)
	assert.Equal(t, "sha256:abcd1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab", entry.Data.Digest)
	assert.Equal(t, "test-package", entry.Data.Name)
	assert.Len(t, entry.Data.Versions, 3)
	assert.Contains(t, entry.Data.Versions, "1.0.0")
	assert.Contains(t, entry.Data.Versions, "1.1.0")
	assert.Contains(t, entry.Data.Versions, "2.0.0")
	assert.Len(t, entry.Data.Constraints, 1)
}

func TestRepositoryEntryData_AllFields(t *testing.T) {
	t.Parallel()

	data := RepositoryEntryData{
		Image:  "docker.io/library/nginx",
		Digest: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		Versions: []string{
			"1.0.0",
			"1.0.1",
			"1.1.0-beta.1",
			"2.0.0-rc.1",
		},
		Name: "nginx-package",
		Constraints: []PackageManifestConstraint{
			{
				Platform: []PlatformName{Kubernetes},
				PlatformVersion: &PackageManifestPlatformVersionConstraint{
					Name:  Kubernetes,
					Range: ">=1.19",
				},
			},
			{
				Platform: []PlatformName{OpenShift},
				PlatformVersion: &PackageManifestPlatformVersionConstraint{
					Name:  OpenShift,
					Range: ">=4.8",
				},
			},
		},
	}

	assert.Equal(t, "docker.io/library/nginx", data.Image)
	assert.Equal(t, "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", data.Digest)
	assert.Equal(t, "nginx-package", data.Name)
	assert.Len(t, data.Versions, 4)
	assert.Len(t, data.Constraints, 2)

	// Test versions include various semver formats
	assert.Contains(t, data.Versions, "1.0.0")
	assert.Contains(t, data.Versions, "1.0.1")
	assert.Contains(t, data.Versions, "1.1.0-beta.1")
	assert.Contains(t, data.Versions, "2.0.0-rc.1")

	// Test constraints
	assert.Equal(t, Kubernetes, data.Constraints[0].Platform[0])
	assert.Equal(t, ">=1.19", data.Constraints[0].PlatformVersion.Range)
	assert.Equal(t, OpenShift, data.Constraints[1].Platform[0])
	assert.Equal(t, ">=4.8", data.Constraints[1].PlatformVersion.Range)
}

func TestRepositoryEntryData_EmptyFields(t *testing.T) {
	t.Parallel()

	data := RepositoryEntryData{}

	assert.Empty(t, data.Image)
	assert.Empty(t, data.Digest)
	assert.Empty(t, data.Name)
	assert.Empty(t, data.Versions)
	assert.Empty(t, data.Constraints)
}

func TestRepositoryEntryData_MinimalFields(t *testing.T) {
	t.Parallel()

	data := RepositoryEntryData{
		Image: "registry.local/minimal-package",
		Name:  "minimal",
	}

	assert.Equal(t, "registry.local/minimal-package", data.Image)
	assert.Equal(t, "minimal", data.Name)
	assert.Empty(t, data.Digest)
	assert.Empty(t, data.Versions)
	assert.Empty(t, data.Constraints)
}

func TestRepositoryEntry_WithComplexConstraints(t *testing.T) {
	t.Parallel()

	entry := &RepositoryEntry{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "manifests.package-operator.run/v1alpha1",
			Kind:       "RepositoryEntry",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "complex-package",
			Labels: map[string]string{
				"category": "database",
				"tier":     "production",
			},
			Annotations: map[string]string{
				"description": "A complex package with multiple constraints",
			},
		},
		Data: RepositoryEntryData{
			Image:    "quay.io/company/complex-package",
			Digest:   "sha256:fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
			Versions: []string{"3.1.4", "3.2.0"},
			Name:     "complex-package",
			Constraints: []PackageManifestConstraint{
				{
					Platform: []PlatformName{OpenShift},
					PlatformVersion: &PackageManifestPlatformVersionConstraint{
						Name:  OpenShift,
						Range: ">=4.10",
					},
					UniqueInScope: &PackageManifestUniqueInScopeConstraint{},
				},
			},
		},
	}

	assert.Equal(t, "complex-package", entry.Name)
	assert.Equal(t, "database", entry.Labels["category"])
	assert.Equal(t, "production", entry.Labels["tier"])
	assert.Equal(t, "A complex package with multiple constraints", entry.Annotations["description"])
	assert.Equal(t, "complex-package", entry.Data.Name)
	assert.Len(t, entry.Data.Constraints, 1)
	assert.NotNil(t, entry.Data.Constraints[0].UniqueInScope)
}
