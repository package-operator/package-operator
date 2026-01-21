package manifests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRepository_Structure(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "manifests.package-operator.run/v1alpha1",
			Kind:       "Repository",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repository",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"type": "package-repository",
			},
			Annotations: map[string]string{
				"description": "Test repository for packages",
			},
		},
	}

	assert.Equal(t, "manifests.package-operator.run/v1alpha1", repo.APIVersion)
	assert.Equal(t, "Repository", repo.Kind)
	assert.Equal(t, "test-repository", repo.Name)
	assert.Equal(t, "test-namespace", repo.Namespace)
	assert.Equal(t, "package-repository", repo.Labels["type"])
	assert.Equal(t, "Test repository for packages", repo.Annotations["description"])
}

func TestRepository_ObjectMeta(t *testing.T) {
	t.Parallel()

	repo := &Repository{}

	// Test setting metadata
	repo.SetName("my-repo")
	repo.SetNamespace("my-namespace")
	repo.SetLabels(map[string]string{
		"app":     "package-operator",
		"version": "v1.0.0",
	})
	repo.SetAnnotations(map[string]string{
		"created-by": "admin",
		"purpose":    "testing",
	})

	assert.Equal(t, "my-repo", repo.GetName())
	assert.Equal(t, "my-namespace", repo.GetNamespace())
	assert.Equal(t, "package-operator", repo.GetLabels()["app"])
	assert.Equal(t, "v1.0.0", repo.GetLabels()["version"])
	assert.Equal(t, "admin", repo.GetAnnotations()["created-by"])
	assert.Equal(t, "testing", repo.GetAnnotations()["purpose"])
}

func TestRepository_TypeMeta(t *testing.T) {
	t.Parallel()

	repo := &Repository{}

	// Test setting type metadata
	repo.TypeMeta = metav1.TypeMeta{
		APIVersion: "manifests.package-operator.run/v1alpha1",
		Kind:       "Repository",
	}

	assert.Equal(t, "manifests.package-operator.run/v1alpha1", repo.APIVersion)
	assert.Equal(t, "Repository", repo.Kind)
}

func TestRepository_EmptyRepository(t *testing.T) {
	t.Parallel()

	repo := &Repository{}

	assert.Empty(t, repo.Name)
	assert.Empty(t, repo.Namespace)
	assert.Empty(t, repo.Labels)
	assert.Empty(t, repo.Annotations)
	assert.Empty(t, repo.Kind)
	assert.Empty(t, repo.APIVersion)
}
