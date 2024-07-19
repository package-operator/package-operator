package packagekickstart

import (
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type namespacedName struct {
	namespace, name string
}

// Parses namespace and name from the given unstructured object.
// The function:
// - validatesthe existence of `.metadata` and that it is a map.
// - defaults an empty `.metadata.namespace` to the string "default".
// - validates that `.metadata.name` is not empty.
// - escapes potential filepath separators in the results by passing them through `escapeFilepathSeparator()`
// - returns namespace and name.
func parseObjectMeta(obj unstructured.Unstructured) (namespacedName, error) {
	metadata, ok := obj.Object["metadata"]
	// Validate that .metadata exists.
	if !ok {
		return namespacedName{}, &ObjectIsMissingMetadataError{obj}
	}
	// Validate that .metdata is a map.
	metamap, ok := metadata.(map[string]interface{})
	if !ok {
		return namespacedName{}, &ObjectIsMissingMetadataError{obj}
	}

	// Validate that .metadata.name exists, is a string and is not empty.
	name, ok := metamap["name"].(string)
	if !ok || name == "" {
		return namespacedName{}, &ObjectIsMissingNameError{obj}
	}

	namespace := obj.GetNamespace()
	// Default empty namespace - this also happens for cluster-scoped GKs
	// since there is no reliable way to look up the scope of a GK.
	if namespace == "" {
		namespace = "default"
	}

	escapedNamespace := escapeFilepathSeparator(namespace)
	escapedName := escapeFilepathSeparator(name)
	return namespacedName{
		namespace: escapedNamespace,
		name:      escapedName,
	}, nil
}

// Replace filepath separators (`/` or `\`) with dashes (`-`), to prevent
// the kickstart from creating files outside of the package directory for
// malformed object namespaces or names in the input.
func escapeFilepathSeparator(input string) string {
	return strings.ReplaceAll(input, string(filepath.Separator), "-")
}

type groupKind struct {
	group, kind string
}

// Parses group and kind from the given unstructured object.
// This function:
//   - validates that the apiVersion in the object contains a version.
//   - does NOT validate that the object contains a kind because this already fails the unmarshalling phase.
//     (There is be a regression test for that case!)
//   - defaults an empty apiGroup to the string "core".
func parseTypeMeta(obj unstructured.Unstructured) (groupKind, error) {
	gvk := obj.GroupVersionKind()
	if gvk.Version == "" {
		return groupKind{}, &ObjectHasInvalidAPIVersionError{obj}
	}

	group := obj.GroupVersionKind().Group
	// Expand core api group.
	if group == "" {
		group = "core"
	}

	return groupKind{
		group: group,
		kind:  obj.GetKind(),
	}, nil
}
