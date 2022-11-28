package packages

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

// Default location for the PackageManifest file.
const PackageManifestFile = "manifest.yaml"

// PackageManifestFileNames to probe for.
var PackageManifestFileNames = []string{
	"manifest.yaml",
	"manifest.yml",
}

var PackageManifestGroupKind = schema.GroupKind{
	Group: manifestsv1alpha1.GroupVersion.Group,
	Kind:  "PackageManifest",
}

func IsYAMLFile(path string) bool {
	return strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml")
}

func IsManifestFile(path string) bool {
	for _, filename := range PackageManifestFileNames {
		if path == filename {
			return true
		}
	}
	return false
}
