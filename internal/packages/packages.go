package packages

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

const (
	// Default location for the PackageManifest file.
	PackageManifestFile = "manifest.yaml"
	// Files suffix for all go template files that need pre-processing.
	// .gotmpl is the suffix that is being used by the go language server gopls.
	// https://go-review.googlesource.com/c/tools/+/363360/7/gopls/doc/features.md#29
	TemplateFileSuffix = ".gotmpl"
)

// PackageManifestFileNames to probe for.
var PackageManifestFileNames = []string{
	"manifest.yaml",
	"manifest.yml",
}

var PackageManifestGroupKind = schema.GroupKind{
	Group: manifestsv1alpha1.GroupVersion.Group,
	Kind:  "PackageManifest",
}

// Is path suffixed by .yml or .yaml.
func IsYAMLFile(path string) bool {
	return strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml")
}

// Is path suffixed by .gotmpl.
func IsTemplateFile(path string) bool {
	return strings.HasSuffix(path, TemplateFileSuffix)
}

// Is the it the manifest file.
func IsManifestFile(path string) bool {
	for _, filename := range PackageManifestFileNames {
		if path == filename {
			return true
		}
	}
	return false
}
