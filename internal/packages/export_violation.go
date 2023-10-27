package packages

import (
	"package-operator.run/internal/packages/internal/packagetypes"
)

type (
	// ViolationError describes the reason why and which part of a package is violating sanitation checks.
	ViolationError = packagetypes.ViolationError
	// ViolationReason describes in short how something violates violating sanitation checks.
	ViolationReason = packagetypes.ViolationReason
)

var (
	// ErrManifestNotFound indicates that a package manifest was not found at any expected location.
	ErrManifestNotFound = packagetypes.ErrManifestNotFound

	ViolationReasonPackageManifestNotFound       = packagetypes.ViolationReasonPackageManifestNotFound
	ViolationReasonPackageManifestUnknownGVK     = packagetypes.ViolationReasonPackageManifestUnknownGVK
	ViolationReasonPackageManifestConversion     = packagetypes.ViolationReasonPackageManifestConversion
	ViolationReasonPackageManifestInvalid        = packagetypes.ViolationReasonPackageManifestInvalid
	ViolationReasonPackageManifestDuplicated     = packagetypes.ViolationReasonPackageManifestDuplicated
	ViolationReasonPackageManifestLockUnknownGVK = packagetypes.ViolationReasonPackageManifestLockUnknownGVK
	ViolationReasonPackageManifestLockConversion = packagetypes.ViolationReasonPackageManifestLockConversion
	ViolationReasonPackageManifestLockInvalid    = packagetypes.ViolationReasonPackageManifestLockInvalid
	ViolationReasonPackageManifestLockDuplicated = packagetypes.ViolationReasonPackageManifestLockDuplicated
	ViolationReasonInvalidYAML                   = packagetypes.ViolationReasonInvalidYAML
	ViolationReasonMissingPhaseAnnotation        = packagetypes.ViolationReasonMissingPhaseAnnotation
	ViolationReasonMissingGVK                    = packagetypes.ViolationReasonMissingGVK
	ViolationReasonDuplicateObject               = packagetypes.ViolationReasonDuplicateObject
	ViolationReasonLabelsInvalid                 = packagetypes.ViolationReasonLabelsInvalid
	ViolationReasonUnsupportedScope              = packagetypes.ViolationReasonUnsupportedScope
	ViolationReasonFixtureMismatch               = packagetypes.ViolationReasonFixtureMismatch
	ViolationReasonComponentsNotEnabled          = packagetypes.ViolationReasonComponentsNotEnabled
	ViolationReasonComponentNotFound             = packagetypes.ViolationReasonComponentNotFound
	ViolationReasonInvalidComponentPath          = packagetypes.ViolationReasonInvalidComponentPath
	ViolationReasonUnknown                       = packagetypes.ViolationReasonUnknown
	ViolationReasonNestedMultiComponentPkg       = packagetypes.ViolationReasonNestedMultiComponentPkg
	ViolationReasonInvalidFileInComponentsDir    = packagetypes.ViolationReasonInvalidFileInComponentsDir
	ViolationReasonKubeconform                   = packagetypes.ViolationReasonKubeconform
)
