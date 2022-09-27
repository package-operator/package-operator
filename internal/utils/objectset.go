package utils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"sigs.k8s.io/yaml"
)

func GetObjectsFromPhases(phases []corev1alpha1.ObjectSetTemplatePhase) []corev1alpha1.ObjectSetObject {
	var result []corev1alpha1.ObjectSetObject
	for _, phase := range phases {
		result = append(result, phase.Objects...)
	}
	return result
}

func UnstructuredFromObjectObject(objectsetObject *corev1alpha1.ObjectSetObject) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	// Warning!
	// This MUST absolutely use sigs.k8s.io/yaml
	// Any other yaml parser, might yield unexpected results.
	if err := yaml.Unmarshal(objectsetObject.Object.Raw, obj); err != nil {
		return nil, fmt.Errorf("converting RawExtension into unstructured: %w", err)
	}
	return obj, nil
}
