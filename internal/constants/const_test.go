package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "DynamicCacheLabel",
			constant: DynamicCacheLabel,
			expected: "package-operator.run/cache",
		},
		{
			name:     "CachedFinalizer",
			constant: CachedFinalizer,
			expected: "package-operator.run/cached",
		},
		{
			name:     "ChangeCauseAnnotation",
			constant: ChangeCauseAnnotation,
			expected: "kubernetes.io/change-cause",
		},
		{
			name:     "ForceAdoptionEnvironmentVariable",
			constant: ForceAdoptionEnvironmentVariable,
			expected: "PKO_FORCE_ADOPTION",
		},
		{
			name:     "FieldOwner",
			constant: FieldOwner,
			expected: "package-operator",
		},
		{
			name:     "OwnerStrategyAnnotationKey",
			constant: OwnerStrategyAnnotationKey,
			expected: "package-operator.run/owners",
		},
		{
			name:     "MetricsFinalizer",
			constant: MetricsFinalizer,
			expected: "package-operator.run/metrics",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.expected, test.constant)
		})
	}
}

func TestLogLevelDebug(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 1, LogLevelDebug)
}

func TestStaticCacheOwner(t *testing.T) {
	t.Parallel()

	owner := StaticCacheOwner()

	assert.NotNil(t, owner)
	assert.IsType(t, &corev1alpha1.ObjectDeployment{}, owner)
	assert.Equal(t, types.UID("123-456"), owner.UID)

	// Test that subsequent calls return the same structure
	owner2 := StaticCacheOwner()
	assert.Equal(t, owner.UID, owner2.UID)
}

func TestStaticCacheOwner_ObjectMeta(t *testing.T) {
	t.Parallel()

	owner := StaticCacheOwner()

	// Verify ObjectMeta structure
	assert.Equal(t, metav1.ObjectMeta{
		UID: "123-456",
	}, owner.ObjectMeta)
}

func TestStaticCacheOwner_ReturnType(t *testing.T) {
	t.Parallel()

	owner := StaticCacheOwner()

	// Verify it's a pointer to ObjectDeployment
	assert.NotNil(t, owner)

	// Verify the underlying type
	_, ok := interface{}(owner).(*corev1alpha1.ObjectDeployment)
	assert.True(t, ok, "StaticCacheOwner should return *corev1alpha1.ObjectDeployment")
}
