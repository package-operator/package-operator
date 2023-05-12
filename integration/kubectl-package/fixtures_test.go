package kubectlpackage

import (
	"path/filepath"
)

func sourcePathFixture(name string) string {
	var (
		packagesDir     = filepath.Join("testdata", "packages")
		invalidPackages = filepath.Join(packagesDir, "invalid")
		validPackages   = filepath.Join(packagesDir, "valid")
	)

	return map[string]string{
		"valid_without_config":             filepath.Join(validPackages, "without_config"),
		"valid_with_config":                filepath.Join(validPackages, "with_config"),
		"invalid_bad_manifest":             filepath.Join(invalidPackages, "bad_manifest"),
		"invalid_invalid_resource_label":   filepath.Join(invalidPackages, "invalid_resource_label"),
		"invalid_missing_lock_file":        filepath.Join(invalidPackages, "missing_lock_file"),
		"invalid_missing_phase_annotation": filepath.Join(invalidPackages, "missing_phase_annotation"),
		"invalid_missing_resource_gvk":     filepath.Join(invalidPackages, "missing_resource_gvk"),
	}[name]
}
