package packagevalidation

import (
	"context"
	"errors"
	"slices"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagemanifestvalidation"
	"package-operator.run/internal/packages/internal/packagetypes"
)

// DefaultPackageValidators is a list of package validators that should be executed as a minimum standard.
var DefaultPackageValidators = PackageValidatorList{
	&PackageManifestValidator{},
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
	return errors.Join(errs...)
}

// Validates PackageManifests and PackageManifestLock.
type PackageManifestValidator struct{}

func (v *PackageManifestValidator) ValidatePackage(ctx context.Context, pkg *packagetypes.Package) error {
	errList, err := packagemanifestvalidation.ValidatePackageManifest(ctx, pkg.Manifest)
	if err != nil {
		return err
	}
	if pkg.ManifestLock != nil {
		lockErrList, err := packagemanifestvalidation.ValidatePackageManifestLock(ctx, pkg.ManifestLock)
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
