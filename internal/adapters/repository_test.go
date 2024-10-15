package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

const (
	expectedHash = "3556c6f8c06393ca4dc900c60e3de1a58f1c2ffc4b3f7d19c0fd2d303aea96d3"
	hash123      = "123"
	test         = "test"
)

func TestGenericRepository(t *testing.T) {
	t.Parallel()
	repo := NewGenericRepository(testScheme)
	assert.True(t, repo.IsNamespaced())

	assert.NotNil(t, repo.ClientObject())
	repo.UpdatePhase()
	r := repo.ClientObject().(*corev1alpha1.Repository)
	assert.Equal(
		t, corev1alpha1.RepositoryPhaseUnpacking, r.Status.Phase)

	r.Spec.Image = test
	assert.Equal(t, r.Spec.Image, repo.GetImage())

	assert.Equal(t, expectedHash, repo.GetSpecHash(nil))

	repo.SetUnpackedHash(hash123)
	assert.Equal(t, hash123, r.Status.UnpackedHash)
	assert.Equal(t, hash123, repo.GetUnpackedHash())

	assert.Empty(t, repo.GetConditions())
	r.Status.Conditions = []metav1.Condition{
		{
			ObservedGeneration: 1,
			Type:               "test-type",
			Reason:             "test-reason",
			Message:            "test-message",
		},
	}
	assert.Equal(t, r.Status.Conditions, *repo.GetConditions())

	var statusPhase corev1alpha1.RepositoryStatusPhase = test
	repo.setStatusPhase(statusPhase)
	assert.Equal(t, statusPhase, r.Status.Phase)
}

func TestGenericClusterRepository(t *testing.T) {
	t.Parallel()
	repo := NewGenericClusterRepository(testScheme)
	assert.False(t, repo.IsNamespaced())

	assert.NotNil(t, repo.ClientObject())
	repo.UpdatePhase()
	r := repo.ClientObject().(*corev1alpha1.ClusterRepository)
	assert.Equal(
		t, corev1alpha1.RepositoryPhaseUnpacking, r.Status.Phase)

	r.Spec.Image = test
	assert.Equal(t, r.Spec.Image, repo.GetImage())

	assert.Equal(t, expectedHash, repo.GetSpecHash(nil))

	repo.SetUnpackedHash(hash123)
	assert.Equal(t, hash123, r.Status.UnpackedHash)
	assert.Equal(t, hash123, repo.GetUnpackedHash())

	assert.Empty(t, repo.GetConditions())
	r.Status.Conditions = []metav1.Condition{
		{
			ObservedGeneration: 1,
			Type:               "test-type",
			Reason:             "test-reason",
			Message:            "test-message",
		},
	}
	assert.Equal(t, r.Status.Conditions, *repo.GetConditions())

	var statusPhase corev1alpha1.RepositoryStatusPhase = test
	repo.setStatusPhase(statusPhase)
	assert.Equal(t, statusPhase, r.Status.Phase)
}

func Test_updateRepositoryPhase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		conditions []metav1.Condition
		expected   corev1alpha1.RepositoryStatusPhase
	}{
		{
			name: "Invalid",
			conditions: []metav1.Condition{
				{
					Type:   corev1alpha1.RepositoryInvalid,
					Status: metav1.ConditionTrue,
				},
			},
			expected: corev1alpha1.RepositoryPhaseInvalid,
		},
		{
			name:       "Unpacking",
			conditions: []metav1.Condition{},
			expected:   corev1alpha1.RepositoryPhaseUnpacking,
		},
		{
			name: "Available",
			conditions: []metav1.Condition{
				{
					Type:   corev1alpha1.RepositoryUnpacked,
					Status: metav1.ConditionTrue,
				},
				{
					Type:   corev1alpha1.RepositoryAvailable,
					Status: metav1.ConditionTrue,
				},
			},
			expected: corev1alpha1.RepositoryPhaseAvailable,
		},
		{
			name: "NotReady",
			conditions: []metav1.Condition{
				{
					Type:   corev1alpha1.RepositoryUnpacked,
					Status: metav1.ConditionTrue,
				},
			},
			expected: corev1alpha1.RepositoryPhaseNotReady,
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			pkg := &GenericRepository{
				Repository: corev1alpha1.Repository{
					Status: corev1alpha1.RepositoryStatus{
						Conditions: test.conditions,
					},
				},
			}
			updateRepositoryPhase(pkg)
			assert.Equal(t, test.expected, pkg.Repository.Status.Phase)
		})
	}
}
