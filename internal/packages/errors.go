package packages

import (
	"fmt"
	"strings"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

// PackageManifestNotFoundError is returned when no manifest.yml can be found.
type PackageManifestNotFoundError struct{}

func (e *PackageManifestNotFoundError) Error() string {
	return "PackageManifest not found, searched for " + strings.Join(packageManifestFileNames, ", ")
}

// PackageManifestInvalidError is returned when the found PackageManifest can not be parsed.
type PackageManifestInvalidError struct {
	Reason string
	Err    error // if this error was caused by another error.
}

func (e *PackageManifestInvalidError) Unwrap() error {
	return e.Err
}

func (e *PackageManifestInvalidError) Error() string {
	out := "PackageManifest invalid: "
	if e.Err != nil {
		return out + e.Err.Error()
	}
	return out + e.Reason
}

type PackageObjectInvalidError struct {
	FilePath string
	Reason   string
}

func (e *PackageObjectInvalidError) Error() string {
	return fmt.Sprintf("Object at %s invalid: %s", e.FilePath, e.Reason)
}

type PackageInvalidScopeError struct {
	RequiredScope   manifestsv1alpha1.PackageManifestScope
	SupportedScopes []manifestsv1alpha1.PackageManifestScope
}

func (e *PackageInvalidScopeError) Error() string {
	scopes := make([]string, len(e.SupportedScopes))
	for i := range e.SupportedScopes {
		scopes[i] = string(e.SupportedScopes[i])
	}
	return fmt.Sprintf(
		"Package does not support %s scope, supported scopes are: %s",
		e.RequiredScope, strings.Join(scopes, ", "))
}
