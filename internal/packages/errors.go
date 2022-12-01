package packages

import (
	"errors"
	"fmt"
	"strings"

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

func NewInvalidAggregate(errorList ...error) error {
	var violations []Violation
	for _, e := range errorList {
		if e == nil {
			continue
		}

		var ie *InvalidError
		if errors.As(e, &ie) {
			violations = append(violations, ie.Violations...)
			continue
		}
		return e
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
	for i, v := range e.Violations {
		if i != 0 {
			msg += "\n"
		}
		msg += "- " + strings.ReplaceAll(v.String(), "\n", "\n  ")
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
	if v.Location != nil {
		msg = fmt.Sprintf("%s in %s", msg, v.Location.String())
	}

	if len(v.Details) > 0 {
		msg += ":\n" + v.Details
	}
	return msg
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
	ViolationReasonMissingGVK                = "GroupVersionKind not set"
	ViolationReasonLabelsInvalid             = "Labels invalid"
	ViolationReasonUnsupportedScope          = "Package unsupported scope"
	ViolationReasonFixtureMismatch           = "File mismatch against fixture"
)
