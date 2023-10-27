package packagerender

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

type conditionMapParseError struct {
	Message    string
	LineNumber int
}

func (e conditionMapParseError) Error() string {
	return e.Message + fmt.Sprintf(" in line %d", e.LineNumber)
}

func parseConditionMapAnnotation(obj *unstructured.Unstructured) ([]corev1alpha1.ConditionMapping, error) {
	conditionMapAnnotation, ok := obj.GetAnnotations()[manifestsv1alpha1.PackageConditionMapAnnotation]
	if !ok {
		return nil, nil
	}

	inputMappings := strings.Split(strings.TrimSpace(conditionMapAnnotation), "\n")
	outputMappings := make([]corev1alpha1.ConditionMapping, len(inputMappings))
	for i, rawMapping := range inputMappings {
		line := i + 1
		parts := strings.SplitN(rawMapping, "=>", 2)
		if len(parts) != 2 {
			return nil, conditionMapParseError{
				Message:    fmt.Sprintf("expected 2 part mapping got %d", len(parts)),
				LineNumber: line,
			}
		}
		if len(parts[0]) == 0 {
			return nil, conditionMapParseError{
				Message:    "sourceType can't be empty",
				LineNumber: line,
			}
		}
		if len(parts[1]) == 0 {
			return nil, conditionMapParseError{
				Message:    "destinationType can't be empty",
				LineNumber: line,
			}
		}

		outputMappings[i] = corev1alpha1.ConditionMapping{
			SourceType:      strings.TrimSpace(parts[0]),
			DestinationType: strings.TrimSpace(parts[1]),
		}
	}

	return outputMappings, nil
}
