package packagestructure

import (
	"fmt"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

type InvalidError struct {
	Violations []Violation
}

func NewInvalidError(violations ...Violation) *InvalidError {
	return &InvalidError{
		Violations: violations,
	}
}

func NewInvalidAggregate(invalidErrors ...*InvalidError) *InvalidError {
	var violations []Violation
	for _, e := range invalidErrors {
		if e == nil {
			continue
		}
		violations = append(violations, e.Violations...)
	}
	if len(violations) == 0 {
		return nil
	}
	return NewInvalidError(violations...)
}

func (e *InvalidError) Error() string {
	if e == nil {
		return ""
	}

	msg := "Package validation errors:\n"
	for _, v := range e.Violations {
		msg += "- " + v.String() + "\n"
	}
	return msg
}

type Violation struct {
	Reason  string
	Details string

	Location *ViolationLocation
}

func (v Violation) String() string {
	msg := v.Reason
	if len(v.Details) > 0 {
		msg += " " + v.Details
	}
	if v.Location == nil {
		return msg
	}

	return fmt.Sprintf("%s in %s", msg, v.Location.String())
}

type ViolationLocation struct {
	Path          string
	DocumentIndex *int
}

func (l *ViolationLocation) String() string {
	if l == nil {
		return ""
	}
	if l.DocumentIndex == nil {
		return l.Path
	}
	return fmt.Sprintf("%s#%d", l.Path, *l.DocumentIndex)
}

const (
	ViolationReasonPackageManifestNotFound   = "PackageManifest not found"
	ViolationReasonPackageManifestUnknownGVK = "PackageManifest unknown GVK"
	ViolationReasonPackageManifestConversion = "PackageManifest conversion"
	ViolationReasonPackageManifestInvalid    = "PackageManifest invalid"
	ViolationReasonInvalidYAML               = "Invalid YAML"
	ViolationReasonMissingPhaseAnnotation    = "Missing " + manifestsv1alpha1.PackagePhaseAnnotation + " Annotation"
	ViolationReasonUnsupportedScope          = "Package unsupported scope"
)
