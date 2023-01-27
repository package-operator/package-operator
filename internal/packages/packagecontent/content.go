package packagecontent

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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
	}
)
