package utils

import (
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func GetObjectsFromPhases(phases []corev1alpha1.ObjectSetTemplatePhase) []corev1alpha1.ObjectSetObject {
	// Calculate total capacity needed
	capacity := 0
	for _, phase := range phases {
		capacity += len(phase.Objects)
	}

	result := make([]corev1alpha1.ObjectSetObject, 0, capacity)
	for _, phase := range phases {
		result = append(result, phase.Objects...)
	}
	return result
}
