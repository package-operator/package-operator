package secretsync

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

var _ adoptionChecker = (*defaultAdoptionChecker)(nil)

type defaultAdoptionChecker struct {
	ownerStrategy ownerStrategy
}

func (c *defaultAdoptionChecker) Check(owner, obj client.Object) (bool, error) {
	if c.ownerStrategy.IsController(owner, obj) {
		// already owner, nothing to do.
		return false, nil
	}

	// Get value of collision protection, defaulting to "Prevent".
	annotations := owner.GetAnnotations()
	cpAnnotation := annotations[manifestsv1alpha1.PackageCollisionProtectionAnnotation]
	if cpAnnotation == "" {
		cpAnnotation = string(corev1alpha1.CollisionProtectionPrevent)
	}

	collisionProtection := corev1alpha1.CollisionProtection(cpAnnotation)
	switch collisionProtection {
	case corev1alpha1.CollisionProtectionPrevent:
		return false, nil
	case corev1alpha1.CollisionProtectionNone:
		return true, nil
	case corev1alpha1.CollisionProtectionIfNoController:
		if !c.ownerStrategy.HasController(obj) {
			return true, nil
		}
	}

	return true, nil
}
