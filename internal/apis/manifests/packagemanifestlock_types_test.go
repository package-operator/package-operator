package manifests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPackageManifestLock_Structure(t *testing.T) {
	t.Parallel()

	lock := &PackageManifestLock{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "manifests.package-operator.run/v1alpha1",
			Kind:       "PackageManifestLock",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-package-lock",
			Namespace: "test-namespace",
		},
		Spec: PackageManifestLockSpec{
			Images: []PackageManifestLockImage{
				{
					Name:   "app",
					Image:  "nginx:1.20",
					Digest: "sha256:abc123",
				},
				{
					Name:   "sidecar",
					Image:  "busybox:latest",
					Digest: "sha256:def456",
				},
			},
			Dependencies: []PackageManifestLockDependency{
				{
					Name:    "database",
					Image:   "quay.io/example/postgres-package",
					Digest:  "sha256:789xyz",
					Version: "v1.2.3",
				},
			},
		},
	}

	assert.Equal(t, "manifests.package-operator.run/v1alpha1", lock.APIVersion)
	assert.Equal(t, "PackageManifestLock", lock.Kind)
	assert.Equal(t, "test-package-lock", lock.Name)
	assert.Equal(t, "test-namespace", lock.Namespace)
	assert.Len(t, lock.Spec.Images, 2)
	assert.Len(t, lock.Spec.Dependencies, 1)
}

func TestPackageManifestLockSpec_Images(t *testing.T) {
	t.Parallel()

	spec := PackageManifestLockSpec{
		Images: []PackageManifestLockImage{
			{
				Name:   "main-app",
				Image:  "docker.io/library/nginx:1.21.6",
				Digest: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			},
			{
				Name:   "init-container",
				Image:  "alpine:3.15",
				Digest: "sha256:fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
			},
		},
	}

	assert.Len(t, spec.Images, 2)

	// Test first image
	assert.Equal(t, "main-app", spec.Images[0].Name)
	assert.Equal(t, "docker.io/library/nginx:1.21.6", spec.Images[0].Image)
	assert.Equal(t, "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", spec.Images[0].Digest)

	// Test second image
	assert.Equal(t, "init-container", spec.Images[1].Name)
	assert.Equal(t, "alpine:3.15", spec.Images[1].Image)
	assert.Equal(t, "sha256:fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321", spec.Images[1].Digest)
}

func TestPackageManifestLockSpec_Dependencies(t *testing.T) {
	t.Parallel()

	spec := PackageManifestLockSpec{
		Dependencies: []PackageManifestLockDependency{
			{
				Name:    "monitoring",
				Image:   "quay.io/prometheus/prometheus",
				Digest:  "sha256:a1b2c3d4e5f67890a1b2c3d4e5f67890a1b2c3d4e5f67890a1b2c3d4e5f67890",
				Version: "v2.35.0",
			},
			{
				Name:    "logging",
				Image:   "docker.elastic.co/elasticsearch/elasticsearch",
				Digest:  "sha256:9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba",
				Version: "8.2.0",
			},
		},
	}

	assert.Len(t, spec.Dependencies, 2)

	// Test first dependency
	assert.Equal(t, "monitoring", spec.Dependencies[0].Name)
	assert.Equal(t, "quay.io/prometheus/prometheus", spec.Dependencies[0].Image)
	assert.Equal(t, "sha256:a1b2c3d4e5f67890a1b2c3d4e5f67890a1b2c3d4e5f67890a1b2c3d4e5f67890", spec.Dependencies[0].Digest)
	assert.Equal(t, "v2.35.0", spec.Dependencies[0].Version)

	// Test second dependency
	assert.Equal(t, "logging", spec.Dependencies[1].Name)
	assert.Equal(t, "docker.elastic.co/elasticsearch/elasticsearch", spec.Dependencies[1].Image)
	assert.Equal(t, "sha256:9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba", spec.Dependencies[1].Digest)
	assert.Equal(t, "8.2.0", spec.Dependencies[1].Version)
}

func TestPackageManifestLockImage_AllFields(t *testing.T) {
	t.Parallel()

	image := PackageManifestLockImage{
		Name:   "web-server",
		Image:  "registry.k8s.io/nginx-slim:0.8",
		Digest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}

	assert.Equal(t, "web-server", image.Name)
	assert.Equal(t, "registry.k8s.io/nginx-slim:0.8", image.Image)
	assert.Equal(t, "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", image.Digest)
}

func TestPackageManifestLockDependency_AllFields(t *testing.T) {
	t.Parallel()

	dependency := PackageManifestLockDependency{
		Name:    "database-operator",
		Image:   "quay.io/example/postgres-operator",
		Digest:  "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		Version: "v1.15.2",
	}

	assert.Equal(t, "database-operator", dependency.Name)
	assert.Equal(t, "quay.io/example/postgres-operator", dependency.Image)
	assert.Equal(t, "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", dependency.Digest)
	assert.Equal(t, "v1.15.2", dependency.Version)
}

func TestPackageManifestLock_EmptySpec(t *testing.T) {
	t.Parallel()

	lock := &PackageManifestLock{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "manifests.package-operator.run/v1alpha1",
			Kind:       "PackageManifestLock",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "empty-lock",
		},
		Spec: PackageManifestLockSpec{},
	}

	assert.Equal(t, "empty-lock", lock.Name)
	assert.Empty(t, lock.Spec.Images)
	assert.Empty(t, lock.Spec.Dependencies)
}

func TestPackageManifestLock_OnlyImages(t *testing.T) {
	t.Parallel()

	lock := &PackageManifestLock{
		Spec: PackageManifestLockSpec{
			Images: []PackageManifestLockImage{
				{
					Name:   "single-app",
					Image:  "alpine:latest",
					Digest: "sha256:abcd1234",
				},
			},
		},
	}

	assert.Len(t, lock.Spec.Images, 1)
	assert.Empty(t, lock.Spec.Dependencies)
	assert.Equal(t, "single-app", lock.Spec.Images[0].Name)
}

func TestPackageManifestLock_OnlyDependencies(t *testing.T) {
	t.Parallel()

	lock := &PackageManifestLock{
		Spec: PackageManifestLockSpec{
			Dependencies: []PackageManifestLockDependency{
				{
					Name:    "external-service",
					Image:   "redis:6.2",
					Digest:  "sha256:xyz789",
					Version: "v6.2.5",
				},
			},
		},
	}

	assert.Empty(t, lock.Spec.Images)
	assert.Len(t, lock.Spec.Dependencies, 1)
	assert.Equal(t, "external-service", lock.Spec.Dependencies[0].Name)
	assert.Equal(t, "v6.2.5", lock.Spec.Dependencies[0].Version)
}
