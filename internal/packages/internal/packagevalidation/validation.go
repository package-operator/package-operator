package packagevalidation

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagemanifestvalidation"
	"package-operator.run/internal/packages/internal/packagerender"
	"package-operator.run/internal/packages/internal/packagetypes"
)

// DefaultPackageValidators is a list of package validators that should be executed as a minimum standard.
var DefaultPackageValidators = PackageValidatorList{
	&PackageManifestValidator{
		validatePackageManifest:     packagemanifestvalidation.ValidatePackageManifest,
		validatePackageManifestLock: packagemanifestvalidation.ValidatePackageManifestLock,
	},
	&LockfileConsistencyValidator{},
	&PackageStaticFilesWithoutTestCasesValidator{},
}

// PackageValidatorList runs a list of validators and joins all errors.
type PackageValidatorList []packagetypes.PackageValidator

func (l PackageValidatorList) ValidatePackage(ctx context.Context, pkg *packagetypes.Package) error {
	var errs []error
	for _, t := range l {
		if err := t.ValidatePackage(ctx, pkg); err != nil {
			errs = append(errs, err)
		}
	}
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return joinErrorsReadable(true, errs...)
	}
}

// Validates PackageManifests and PackageManifestLock.
type PackageManifestValidator struct {
	validatePackageManifest     func(context.Context, *manifests.PackageManifest) (field.ErrorList, error)
	validatePackageManifestLock func(context.Context, *manifests.PackageManifestLock) (field.ErrorList, error)
}

func (v *PackageManifestValidator) ValidatePackage(ctx context.Context, pkg *packagetypes.Package) error {
	return packagetypes.ValidateEachComponent(ctx, pkg, v.doValidatePackage)
}

func (v *PackageManifestValidator) doValidatePackage(ctx context.Context, pkg *packagetypes.Package, _ bool) error {
	errList, err := v.validatePackageManifest(ctx, pkg.Manifest)
	if err != nil {
		return err
	}
	if pkg.ManifestLock != nil {
		lockErrList, err := v.validatePackageManifestLock(ctx, pkg.ManifestLock)
		if err != nil {
			return err
		}
		errList = append(errList, lockErrList...)
	}
	return errList.ToAggregate()
}

// Validates a Package is able to be installed in the given scope.
type PackageScopeValidator manifests.PackageManifestScope

func (scope PackageScopeValidator) ValidatePackage(_ context.Context, pkg *packagetypes.Package) error {
	if !slices.Contains(pkg.Manifest.Spec.Scopes, manifests.PackageManifestScope(scope)) {
		// Package does not support installation in this scope.
		return packagetypes.ViolationError{
			Reason: packagetypes.ViolationReasonUnsupportedScope,
			Path:   packagetypes.PackageManifestFilename + ".yaml",
		}
	}

	return nil
}

// Validates static files if no test-cases are defined.
type PackageStaticFilesWithoutTestCasesValidator struct{}

func (v PackageStaticFilesWithoutTestCasesValidator) ValidatePackage(
	ctx context.Context, pkg *packagetypes.Package,
) error {
	if len(pkg.Manifest.Test.Template) != 0 {
		return nil
	}

	// Call render objects to validate all static objects.
	if _, err := packagerender.RenderObjects(
		ctx, pkg, packagetypes.PackageRenderContext{},
		DefaultObjectValidators,
	); err != nil {
		return fmt.Errorf("loading package from files: %w", err)
	}
	return nil
}
