package dynamiccache

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// Ensures that the given `obj` is an *unstructured.Unstructured,
// by passing `obj` through `runtime.DefaultUnstructuredConverter` if needed.
// The returned bool signals if the given object had to be converted or not.
func ensureUnstructured(obj runtime.Object, scheme *runtime.Scheme) (*unstructured.Unstructured, bool, error) {
	if uns, ok := obj.(*unstructured.Unstructured); ok {
		return uns, false, nil
	}

	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return nil, false, fmt.Errorf("getting GVK for object: %w", err)
	}

	unsMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, false, fmt.Errorf("converting to unstructured: %w", err)
	}

	uns := &unstructured.Unstructured{Object: unsMap}
	uns.SetGroupVersionKind(gvk)

	return uns, true, nil
}

// Ensures that the given `list` is an *unstructured.UnstructuredList,
// by passing `list` through `runtime.DefaultUnstructuredConverter` if needed.
// The returned bool signals if the given list had to be converted or not.
func ensureUnstructuredList(list client.ObjectList, scheme *runtime.Scheme) (
	*unstructured.UnstructuredList, bool, error,
) {
	if uns, ok := list.(*unstructured.UnstructuredList); ok {
		return uns, false, nil
	}

	gvk, err := apiutil.GVKForObject(list, scheme)
	if err != nil {
		return nil, false, fmt.Errorf("getting GVK for object list: %w", err)
	}

	unsMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(list)
	if err != nil {
		return nil, false, fmt.Errorf("converting to unstructured: %w", err)
	}

	uns := &unstructured.UnstructuredList{Object: unsMap}
	uns.SetGroupVersionKind(gvk)

	return uns, true, nil
}

func toStructured(in *unstructured.Unstructured, out runtime.Object) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(in.Object, out)
}

func toStructuredList(in *unstructured.UnstructuredList, out client.ObjectList) error {
	// This function contains separate handling different storage locations of the list imems.
	// The only other workaround I found, was marshaling the unstructured input into
	// json/yaml and then remarshaling into `out`.

	if len(in.Items) != 0 {
		// The conversion input MUST be supplied via `in.UnstructuredContent()`,
		// because the list items are in `in.Items` and not in `in.Object["items"]`.
		return runtime.DefaultUnstructuredConverter.FromUnstructured(in.UnstructuredContent(), out)
	}

	// The conversion input MUST be supplied via `in.Object`,
	// because the list items are in `in.Object["items"]`.
	return runtime.DefaultUnstructuredConverter.FromUnstructured(in.Object, out)
}
