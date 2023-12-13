package packagetypes

import (
	"fmt"
	"strings"

	"package-operator.run/internal/apis/manifests"
)

// ViolationError describes the reason why and which part of a package is violating sanitation checks.
type ViolationError struct {
	Reason    ViolationReason // Reason is a [ViolationReason] which shortly describes what the reason of this error is.
	Details   string          // Details describes the violation and what to do against it in a more verbose matter.
	Path      string          // Path shows which file path in the package is responsible for this error.
	Component string          // Component indicates which component the error is associated with
	Index     *int            // Index is the index of the YAML document within Path.
	Subject   string          // Complete subject that produced the error, may be the whole yaml file, a single document, etc.
}

func (v ViolationError) Error() string {
	// Set reason to unknown if it is not set.
	if v.Reason == "" {
		v.Reason = ViolationReasonUnknown
	}

	// Always report the reason.
	msg := string(v.Reason)

	// Attach path to message if set.
	if v.Path != "" {
		msg += fmt.Sprintf(" in %s", v.Path)
		if v.Index != nil {
			msg += fmt.Sprintf(" idx %d", *v.Index)
		}
	}

	if v.Component != "" {
		msg += fmt.Sprintf(" [%s]", v.Component)
	}

	// Attach details to message if set.
	if v.Details != "" {
		msg += ": " + v.Details
	}

	if v.Subject != "" {
		msg += "\n" + strings.TrimSpace(v.Subject)
	}

	return msg
}

// ViolationReason describes in short how something violates violating sanitation checks.
type ViolationReason string

// Predefined reasons for package violations.
const (
	ViolationReasonEmptyPackage                  ViolationReason = "Package image contains no files. Might be corrupted."
	ViolationReasonPackageManifestNotFound       ViolationReason = "PackageManifest not found"
	ViolationReasonUnknownGVK                    ViolationReason = "unknown GVK"
	ViolationReasonPackageManifestInvalid        ViolationReason = "PackageManifest invalid"
	ViolationReasonPackageManifestDuplicated     ViolationReason = "PackageManifest present multiple times"
	ViolationReasonPackageManifestLockInvalid    ViolationReason = "PackageManifestLock invalid"
	ViolationReasonPackageManifestLockDuplicated ViolationReason = "PackageManifestLock present multiple times"
	ViolationReasonInvalidYAML                   ViolationReason = "Invalid YAML"
	ViolationReasonMissingPhaseAnnotation        ViolationReason = "Missing " + manifests.PackagePhaseAnnotation + " Annotation"
	ViolationReasonMissingGVK                    ViolationReason = "GroupVersionKind not set"
	ViolationReasonDuplicateObject               ViolationReason = "Duplicate Object"
	ViolationReasonLabelsInvalid                 ViolationReason = "Labels invalid"
	ViolationReasonUnsupportedScope              ViolationReason = "Package unsupported scope"
	ViolationReasonFixtureMismatch               ViolationReason = "File mismatch against fixture"
	ViolationReasonComponentsNotEnabled          ViolationReason = "Components not enabled"
	ViolationReasonComponentNotFound             ViolationReason = "Component not found"
	ViolationReasonInvalidComponentPath          ViolationReason = "Invalid component path"
	ViolationReasonUnknown                       ViolationReason = "Unknown reason"
	ViolationReasonNestedMultiComponentPkg       ViolationReason = "Nesting multi-component packages not allowed"
	ViolationReasonInvalidFileInComponentsDir    ViolationReason = "The components directory may only contain folders and dot files"
	ViolationReasonKubeconform                   ViolationReason = "Kubeconform rejected schema"
)

var ErrEmptyPackage = ViolationError{
	Reason: ViolationReasonEmptyPackage,
}

// ErrManifestNotFound indicates that a package manifest was not found at any expected location.
var ErrManifestNotFound = ViolationError{
	Reason:  ViolationReasonPackageManifestNotFound,
	Details: fmt.Sprintf("searched at %s.yaml and %s.yml", PackageManifestFilename, PackageManifestFilename),
}
