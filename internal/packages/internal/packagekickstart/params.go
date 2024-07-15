package packagekickstart

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// var deploymentGVK = schema.GroupVersionKind{
// 	Group:   "apps",
// 	Version: "v1",
// 	Kind:    "Deployment",
// }

func addParameters(obj unstructured.Unstructured) error {
	switch obj.GetObjectKind().GroupVersionKind() {
	case schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}:
		// unstructured.
		setNestedFieldNoCopy(obj.Object, templateLiteral("{{.config.bananaReplicas}}"), "spec", "replicas")

	}
	return nil
}

type templateLiteral string

func (l templateLiteral) MarshalYAML() ([]byte, error) {
	return []byte(l), nil
}

func setNestedFieldNoCopy(obj map[string]interface{}, value interface{}, fields ...string) error {
	m := obj

	for i, field := range fields[:len(fields)-1] {
		if val, ok := m[field]; ok {
			if valMap, ok := val.(map[string]interface{}); ok {
				m = valMap
			} else {
				return fmt.Errorf("value cannot be set because %v is not a map[string]interface{}", jsonPath(fields[:i+1]))
			}
		} else {
			newVal := make(map[string]interface{})
			m[field] = newVal
			m = newVal
		}
	}
	m[fields[len(fields)-1]] = value
	return nil
}

func jsonPath(fields []string) string {
	return "." + strings.Join(fields, ".")
}
