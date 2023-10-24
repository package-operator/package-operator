package packagetypes

import (
	"path/filepath"
	"strings"
)

// templateFilenameSuffix is the files suffix for all go template files that need pre-processing.
// .gotmpl is the suffix that is being used by the go language server gopls.
// https://go-review.googlesource.com/c/tools/+/363360/7/gopls/doc/features.md#29
const templateFilenameSuffix = ".gotmpl"

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
