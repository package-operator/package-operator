package boxcutterutil

import (
	"pkg.package-operator.run/boxcutter/machinery"
	"pkg.package-operator.run/boxcutter/machinery/types"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TranslateCollisionProtection(in corev1alpha1.CollisionProtection) types.WithCollisionProtection {
	switch in {
	case corev1alpha1.CollisionProtectionNone:
		return types.WithCollisionProtection(types.CollisionProtectionNone)
	case corev1alpha1.CollisionProtectionPrevent:
		return types.WithCollisionProtection(types.CollisionProtectionPrevent)
	case corev1alpha1.CollisionProtectionIfNoController:
		return types.WithCollisionProtection(types.CollisionProtectionIfNoController)
	default:
		panic("TranslateCollisionProtection called without a valid corev1alpha1.CollisionProtection")
	}
}

// TODO get list of controlled objects directly from result?
func GetControllerOf(result machinery.PhaseResult) []corev1alpha1.ControlledObjectReference {
	objects := result.GetObjects()
	controllerOf := make([]corev1alpha1.ControlledObjectReference, 0, len(objects))
	for _, object := range objects {
		// TODO: success is not correct here.
		if !object.Success() {
			continue
		}

		controllerOf = append(controllerOf, corev1alpha1.ControlledObjectReference{
			Kind:      object.Object().GetObjectKind().GroupVersionKind().Kind,
			Group:     object.Object().GetObjectKind().GroupVersionKind().Group,
			Name:      object.Object().GetName(),
			Namespace: object.Object().GetNamespace(),
		})
	}
	return controllerOf
}
