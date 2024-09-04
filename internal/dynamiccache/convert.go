package dynamiccache

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensures that the given `obj` is an *unstructured.Unstructured,
// by passing `obj` through `runtime.DefaultUnstructuredConverter` if needed.
// The returned bool signals if the given object had to be converted or not.
func ensureUnstructured(obj runtime.Object) (*unstructured.Unstructured, bool, error) {
	if uns, ok := obj.(*unstructured.Unstructured); ok {
		return uns, false, nil
	}

	uns, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, false, fmt.Errorf("converting to unstructured: %w", err)
	}

	return &unstructured.Unstructured{Object: uns}, true, nil
}

// Ensures that the given `list` is an *unstructured.UnstructuredList,
// by passing `list` through `runtime.DefaultUnstructuredConverter` if needed.
// The returned bool signals if the given list had to be converted or not.
func ensureUnstructuredList(list client.ObjectList) (*unstructured.UnstructuredList, bool, error) {
	if uns, ok := list.(*unstructured.UnstructuredList); ok {
		return uns, false, nil
	}

	uns, err := runtime.DefaultUnstructuredConverter.ToUnstructured(list)
	if err != nil {
		return nil, false, fmt.Errorf("converting to unstructured: %w", err)
	}

	return &unstructured.UnstructuredList{Object: uns}, true, nil
}

func toStructured(in *unstructured.Unstructured, out runtime.Object) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(in.Object, out)
}

func toStructuredList(in *unstructured.UnstructuredList, out client.ObjectList) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(in.Object, out)
}
