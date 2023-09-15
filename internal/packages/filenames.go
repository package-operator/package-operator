package packages

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	// Default location for the manifest of a package file.
	PackageManifestFilename = "manifest.yaml"

	// Default location for the package lock file.
	PackageManifestLockFilename = "manifest.lock.yaml"

	// templateFilenameSuffix is the files suffix for all go template files that need pre-processing.
	// .gotmpl is the suffix that is being used by the go language server gopls.
	// https://go-review.googlesource.com/c/tools/+/363360/7/gopls/doc/features.md#29
	templateFilenameSuffix = ".gotmpl"
)

// ErrManifestNotFound indicates that a package manifest was not found at any expected location.
var ErrManifestNotFound = ViolationError{
	Reason:  ViolationReasonPackageManifestNotFound,
	Details: fmt.Sprintf("searched at %s and manifest.yml", PackageManifestFilename),
}

// Is path suffixed by [TemplateFileSuffix].
func IsTemplateFile(path string) bool { return strings.HasSuffix(path, templateFilenameSuffix) }

// StripTemplateSuffix removes a [TemplateFileSuffix] suffix from a string if present.
func StripTemplateSuffix(path string) string { return strings.TrimSuffix(path, templateFilenameSuffix) }

// IsYAMLFile return true if the given fileName is suffixed by .yml or .yaml.
func IsYAMLFile(fileName string) bool {
	switch filepath.Ext(fileName) {
	case ".yml", ".yaml":
		return true
	default:
		return false
	}
}

// IsManifestFile returns true if the given file name is considered a package manifest.
func IsManifestFile(fileName string) bool {
	base := filepath.Base(fileName)
	return base == PackageManifestFilename || base == "manifest.yml"
}

// IsManifestFile returns true if the given file name is considered a package manifest lock file.
func IsManifestLockFile(fileName string) bool {
	base := filepath.Base(fileName)
	return base == PackageManifestLockFilename || base == "manifest.lock.yml"
}
