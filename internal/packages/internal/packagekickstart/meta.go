package packagekickstart

import (
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Parses namespace and name from the given unstructured object.
// The function:
// - validatesthe existence of `.metadata` and that it is a map.
// - defaults an empty `.metadata.namespace` to the string "default".
// - validates that `.metadata.name` is not empty.
// - escapes potential filepath separators in the results by passing them through `escapeFilepathSeparator()`
// - returns namespace and name.
func parseObjectMeta(obj unstructured.Unstructured) (client.ObjectKey, error) {
	metadata, ok := obj.Object["metadata"]
	// Validate that .metadata exists.
	if !ok {
		return client.ObjectKey{}, &ObjectIsMissingMetadataError{obj}
	}
	// Validate that .metadata is a map.
	metamap, ok := metadata.(map[string]any)
	if !ok {
		return client.ObjectKey{}, &ObjectIsMissingMetadataError{obj}
	}

	// Validate that .metadata.name exists, is a string and is not empty.
	name, ok := metamap["name"].(string)
	if !ok || name == "" {
		return client.ObjectKey{}, &ObjectIsMissingNameError{obj}
	}

	escapedNamespace := escapeFilepathSeparator(obj.GetNamespace())
	escapedName := escapeFilepathSeparator(name)
	return client.ObjectKey{
		Namespace: escapedNamespace,
		Name:      escapedName,
	}, nil
}

// Replace filepath separators (`/` or `\`) with dashes (`-`), to prevent
// the kickstart from creating files outside of the package directory for
// malformed object namespaces or names in the input.
func escapeFilepathSeparator(input string) string {
	return strings.ReplaceAll(input, string(filepath.Separator), "-")
}

// Parses group and kind from the given unstructured object.
// This function:
//   - validates that the apiVersion in the object contains a version.
//   - does NOT validate that the object contains a kind because this already fails the unmarshalling phase.
//     (There is be a regression test for that case!)
//   - defaults an empty apiGroup to the string "core".
func parseTypeMeta(obj unstructured.Unstructured) (schema.GroupKind, error) {
	gvk := obj.GroupVersionKind()
	if gvk.Version == "" {
		return schema.GroupKind{}, &ObjectHasInvalidAPIVersionError{obj}
	}

	group := obj.GroupVersionKind().Group
	// Expand core api group.
	if group == "" {
		group = "core"
	}

	return schema.GroupKind{
		Group: group,
		Kind:  obj.GetKind(),
	}, nil
}
