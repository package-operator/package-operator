package packagekickstart

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type ObjectIsMissingMetadataError struct {
	obj unstructured.Unstructured
}

func (e *ObjectIsMissingMetadataError) Error() string {
	b, err := yaml.Marshal(e.obj.Object)
	if err != nil {
		b = []byte("Failed to marshal object.")
	}
	return fmt.Sprintf("object is missing metadata: '%s'", b)
}

type ObjectIsMissingNameError struct {
	obj unstructured.Unstructured
}

func (e *ObjectIsMissingNameError) Error() string {
	b, err := yaml.Marshal(e.obj.Object)
	if err != nil {
		b = []byte("Failed to marshal object.")
	}
	return fmt.Sprintf("object is missing name: '%s'", b)
}

type ObjectIsDuplicateError struct {
	obj unstructured.Unstructured
}

func (e *ObjectIsDuplicateError) Error() string {
	b, err := yaml.Marshal(e.obj.Object)
	if err != nil {
		b = []byte("Failed to marshal object.")
	}
	return fmt.Sprintf("duplicate object: '%s'", b)
}

type ObjectHasInvalidAPIVersionError struct {
	obj unstructured.Unstructured
}

func (e *ObjectHasInvalidAPIVersionError) Error() string {
	b, err := yaml.Marshal(e.obj.Object)
	if err != nil {
		b = []byte("Failed to marshal object.")
	}
	return fmt.Sprintf("object has invalid apiVersion: '%s'", b)
}
