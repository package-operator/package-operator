package packages

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

const (
	// Default location for the PackageManifest file.
	PackageManifestFile = "manifest.yaml"

	// Default location for the PackageManifestLock file.
	PackageManifestLockFile = "manifest-lock.yaml"

	// Files suffix for all go template files that need pre-processing.
	// .gotmpl is the suffix that is being used by the go language server gopls.
	// https://go-review.googlesource.com/c/tools/+/363360/7/gopls/doc/features.md#29
	TemplateFileSuffix = ".gotmpl"

	// ImageFilePrefixPath defines under which subfolder files within a package container should be located.
	ImageFilePrefixPath = "package"
)

var (
	// PackageManifestFileNames to probe for.
	PackageManifestFileNames = []string{"manifest.yaml", "manifest.yml"}
	PackageManifestGroupKind = schema.GroupKind{Group: manifestsv1alpha1.GroupVersion.Group, Kind: "PackageManifest"}

	PackageManifestLockFileNames = []string{"manifest-lock.yaml"}
	PackageManifestLockGroupKind = schema.GroupKind{Group: manifestsv1alpha1.GroupVersion.Group, Kind: "PackageManifestLock"}
)

// Is path suffixed by .gotmpl.
func IsTemplateFile(path string) bool { return strings.HasSuffix(path, TemplateFileSuffix) }

// StripTemplateSuffix removes a .gotmpl suffix from a string if present.
func StripTemplateSuffix(path string) string { return strings.TrimSuffix(path, TemplateFileSuffix) }

// Is path suffixed by .yml or .yaml.
func IsYAMLFile(path string) bool {
	return strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml")
}

// Is the it the manifest file.
func IsManifestFile(path string) bool {
	return isAllowedFile(path, PackageManifestFileNames)
}

// Is the it the manifest file.
func IsManifestLockFile(path string) bool {
	return isAllowedFile(path, PackageManifestLockFileNames)
}

func isAllowedFile(path string, allowedValues []string) bool {
	for _, filename := range allowedValues {
		if path == filename {
			return true
		}
	}
	return false
}
