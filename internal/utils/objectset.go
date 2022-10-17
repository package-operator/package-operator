package utils

import (
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func GetObjectsFromPhases(phases []corev1alpha1.ObjectSetTemplatePhase) []corev1alpha1.ObjectSetObject {
	var result []corev1alpha1.ObjectSetObject
	for _, phase := range phases {
		result = append(result, phase.Objects...)
	}
	return result
}
