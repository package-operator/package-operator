// Package constants contains various constant string values for the project.
// They live in a separate package to avoid circular dependencies between packages that contain functional code.
package constants

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

const (
	// DynamicCacheLabel is set on all dynamic objects to limit caches.
	DynamicCacheLabel = "package-operator.run/cache"
	// CachedFinalizer is a common finalizer to free allocated caches when objects are deleted.
	CachedFinalizer = "package-operator.run/cached"
	// ChangeCauseAnnotation records cause of change for history keeping.
	ChangeCauseAnnotation = "kubernetes.io/change-cause"
	// ForceAdoptionEnvironmentVariable causes PKO to skip ownership checks, used during self-bootstrap.
	ForceAdoptionEnvironmentVariable = "PKO_FORCE_ADOPTION"
	// FieldOwner name of the PKO field manager for server-side apply.
	FieldOwner = "package-operator"
	// TODO
	SystemPrefix = "package-operator.run"
	// OwnerStrategyAnnotationKey is the k8s annotation key that denotes the owner of a resource.
	OwnerStrategyAnnotationKey = "package-operator.run/owners"
)

func StaticCacheOwner() *corev1alpha1.ObjectDeployment {
	return &corev1alpha1.ObjectDeployment{
		ObjectMeta: metav1.ObjectMeta{
			UID: "123-456",
		},
	}
}
