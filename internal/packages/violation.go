package packages

import (
	"fmt"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

// ViolationReason describes in short how something violates violating sanitation checks.
type ViolationReason string

// ViolationError describes the reason why and which part of a package is violating sanitation checks.
type ViolationError struct {
	Reason    ViolationReason // Reason is a [ViolationReason] which shortly describes what the reason of this error is.
	Details   string          // Details describes the violation and what to do against it in a more verbose matter.
	Path      string          // Path shows which file path in the package is responsible for this error.
	Component string          // Component indicates which component the error is associated with
	Index     *int            // Index is the index of the YAML document within Path.
}

// Predefined reasons for package violations.
const (
	ViolationReasonPackageManifestNotFound       ViolationReason = "PackageManifest not found"
	ViolationReasonPackageManifestUnknownGVK     ViolationReason = "PackageManifest unknown GVK"
	ViolationReasonPackageManifestConversion     ViolationReason = "PackageManifest conversion"
	ViolationReasonPackageManifestInvalid        ViolationReason = "PackageManifest invalid"
	ViolationReasonPackageManifestDuplicated     ViolationReason = "PackageManifest present multiple times"
	ViolationReasonPackageManifestLockUnknownGVK ViolationReason = "PackageManifestLock unknown GVK"
	ViolationReasonPackageManifestLockConversion ViolationReason = "PackageManifestLock conversion"
	ViolationReasonPackageManifestLockInvalid    ViolationReason = "PackageManifestLock invalid"
	ViolationReasonPackageManifestLockDuplicated ViolationReason = "PackageManifestLock present multiple times"
	ViolationReasonInvalidYAML                   ViolationReason = "Invalid YAML"
	ViolationReasonMissingPhaseAnnotation        ViolationReason = "Missing " + manifestsv1alpha1.PackagePhaseAnnotation + " Annotation"
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
)

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

	return msg
}

// Index creates a *int from the given int parameter to be used in [ViolationError].
func Index(i int) *int { return &i }
