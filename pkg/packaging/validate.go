package packaging

import (
	"context"
	"fmt"

	"package-operator.run/internal/packages"
)

type (
	// ObjectValidator knows how to validate objects within a Package.
	ObjectValidator = packages.ObjectValidator
	// PackageValidator knows how to validate Packages.
	PackageValidator = packages.PackageValidator
)

// ValidateOptions options for the Validate function.
type ValidateOptions struct {
	PackageValidators []PackageValidator
	ObjectValidators  []ObjectValidator
}

// ValidateOption is implemented by Validate options.
type ValidateOption interface {
	ApplyToValidate(opts *ValidateOptions)
}

// WithPackageValidators adds additional PackageValidators to the Package validation process.
type WithPackageValidators []PackageValidator

// ApplyToValidate implements ValidateOption.
func (w WithPackageValidators) ApplyToValidate(opts *ValidateOptions) {
	opts.PackageValidators = w
}

// WithObjectValidators adds additional ObjectValidators to the Package validation process.
type WithObjectValidators []ObjectValidator

// ApplyToValidate implements ValidateOption.
func (w WithObjectValidators) ApplyToValidate(opts *ValidateOptions) {
	opts.ObjectValidators = w
}

// Validate a package use the same presets as kubectl-package.
func Validate(ctx context.Context, pkg *Package, opts ...ValidateOption) error {
	var vopts ValidateOptions
	for _, opt := range opts {
		opt.ApplyToValidate(&vopts)
	}
	var pkgValidators packages.PackageValidatorList = append(
		vopts.PackageValidators, packages.DefaultPackageValidators...)
	var objValidators packages.ObjectValidatorList = append(
		vopts.ObjectValidators, packages.DefaultObjectValidators...)

	// Runs Package-level validations to test structure, settings and templated files.
	if err := pkgValidators.ValidatePackage(ctx, pkg); err != nil {
		return fmt.Errorf("validate package: %w", err)
	}

	// Run object-level validators on non-templated files.
	// Note: RenderObjects only reads already templated .yaml or .yml files.
	if _, err := packages.RenderObjects(
		ctx, pkg, packages.PackageRenderContext{}, objValidators,
	); err != nil {
		return fmt.Errorf("validate package static manifests: %w", err)
	}
	return nil
}
