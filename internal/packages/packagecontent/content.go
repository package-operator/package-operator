package packagecontent

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

type (
	// Files maps filenames to file contents.
	Files map[string][]byte

	// PackageContent represents the parsed content of an PKO package.
	Package struct {
		PackageManifest     *manifestsv1alpha1.PackageManifest
		PackageManifestLock *manifestsv1alpha1.PackageManifestLock
		Objects             map[string][]unstructured.Unstructured
		Permissions         PackagePermissions
	}

	PackagePermissions struct {
		// ObjectTypes managed by this package.
		Managed []schema.GroupKind `json:"managed"`
		// ObjectTypes external to the package, that are included for evaluating status.
		External []schema.GroupKind `json:"external"`
	}
)

// Returns a deep copy of the files map.
func (f Files) DeepCopy() Files {
	newF := Files{}
	for k, v := range f {
		newV := make([]byte, len(v))
		copy(newV, v)
		newF[k] = newV
	}
	return newF
}
