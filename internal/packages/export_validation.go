package packages

import (
	"package-operator.run/internal/packages/packagemanifestvalidation"
	"package-operator.run/internal/packages/packagetypes"
	"package-operator.run/internal/packages/packagevalidation"
)

type (
	// PackageValidator knows how to validate Packages.
	PackageValidator = packagetypes.PackageValidator
	// PackageValidatorList runs a list of validators and joins all errors.
	PackageValidatorList = packagevalidation.PackageValidatorList
	// Validates PackageManifests and PackageManifestLock.
	PackageManifestValidator = packagevalidation.PackageManifestValidator
	// Validates a Package is able to be installed in the given scope.
	PackageScopeValidator = packagevalidation.PackageScopeValidator
	// Runs the template test suites.
	TemplateTestValidator = packagevalidation.TemplateTestValidator

	// ObjectValidator knows how to validate objects within a Package.
	ObjectValidator = packagetypes.ObjectValidator
	// ObjectValidatorList runs a list of validators and joins all errors.
	ObjectValidatorList = packagevalidation.ObjectValidatorList
	// Validates that the PKO phase-annotation is set on all objects.
	ObjectPhaseAnnotationValidator = packagevalidation.ObjectPhaseAnnotationValidator
	// Validates that Objects with the same name/namespace/kind/group must only exist once over all phases.
	// APIVersion does not matter for the check.
	ObjectDuplicateValidator = packagevalidation.ObjectDuplicateValidator
	// Validates that every object has Group, Version and Kind set.
	// e.g. apiVersion: and kind:.
	ObjectGVKValidator = packagevalidation.ObjectGVKValidator
	// Validates that all labels are valid.
	ObjectLabelsValidator = packagevalidation.ObjectLabelsValidator

	// Function given to ValidateEachObject to validate individual objects in a package.
	ValidateEachObjectFn = packagevalidation.ValidateEachObjectFn
)

var (
	// Validates configuration against the PackageManifests OpenAPISchema.
	ValidatePackageConfiguration = packagemanifestvalidation.ValidatePackageConfiguration
	// Validates and Defaults configuration against the PackageManifests OpenAPISchema so it's ready to be used.
	AdmitPackageConfiguration = packagemanifestvalidation.AdmitPackageConfiguration

	// Validates the PackageManifest.
	ValidatePackageManifest = packagemanifestvalidation.ValidatePackageManifest
	// Validates the PackageManifestLock.
	ValidatePackageManifestLock = packagemanifestvalidation.ValidatePackageManifestLock

	// A default list of object validators that should be executed as a minimum standard.
	DefaultObjectValidators = packagevalidation.DefaultObjectValidators
	// A default list of package validators that should be executed as a minimum standard.
	DefaultPackageValidators = packagevalidation.DefaultPackageValidators

	// ValidateEachObject iterates over each object in a package and runs the given validation function.
	ValidateEachObject = packagevalidation.ValidateEachObject

	// Creates a new TemplateTestValidator instance.
	NewTemplateTestValidator = packagevalidation.NewTemplateTestValidator
)
