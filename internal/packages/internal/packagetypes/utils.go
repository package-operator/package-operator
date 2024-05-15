package packagetypes

import (
	"bytes"
	"path/filepath"
	"regexp"
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

var splitYAMLDocumentsRegEx = regexp.MustCompile(`(?m)^---$`)

// Splits a YAML file into multiple documents.
func SplitYAMLDocuments(file []byte) (docs [][]byte) {
	for _, yamlDocument := range splitYAMLDocumentsRegEx.Split(string(bytes.Trim(file, "---\n")), -1) {
		docs = append(docs, bytes.TrimSpace([]byte(yamlDocument)))
	}
	return docs
}

// Joins multiple YAML documents together.
func JoinYAMLDocuments(documents [][]byte) []byte {
	return append(bytes.Join(documents, []byte("\n---\n")), []byte("\n")...)
}
