package packagestructure

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/utils"
)

type Validator interface {
	Validate(ctx context.Context, packageContent *PackageContent) *InvalidError
}

var (
	_ Validator = (ValidatorList)(nil)
	_ Validator = (*ObjectPhaseAnnotationValidator)(nil)
	_ Validator = (PackageScopeValidator)("")

	// DefaultValidators that don't require any inputs.
	DefaultValidators = ValidatorList{
		&ObjectGVKValidator{},
		&ObjectLabelsValidator{},
		&ObjectPhaseAnnotationValidator{},
	}
)

// Runs a list of Validator over the given content.
type ValidatorList []Validator

func (l ValidatorList) Validate(ctx context.Context, packageContent *PackageContent) *InvalidError {
	var errors []*InvalidError
	for _, t := range l {
		if err := t.Validate(ctx, packageContent); err != nil {
			errors = append(errors, err)
		}
	}
	return NewInvalidAggregate(errors...)
}

type ValidateEachObjectFn func(
	ctx context.Context, path string, index int, obj unstructured.Unstructured,
) *InvalidError

func ValidateEachObject(ctx context.Context, mm ManifestMap, validate ValidateEachObjectFn) *InvalidError {
	var errors []*InvalidError
	for path, objects := range mm {
		for i, object := range objects {
			if err := validate(ctx, path, i, object); err != nil {
				errors = append(errors, err)
			}
		}
	}
	return NewInvalidAggregate(errors...)
}

type ObjectPhaseAnnotationValidator struct{}

func (v *ObjectPhaseAnnotationValidator) Validate(ctx context.Context, packageContent *PackageContent) *InvalidError {
	return ValidateEachObject(ctx, packageContent.Manifests, v.validate)
}

func (*ObjectPhaseAnnotationValidator) validate(
	ctx context.Context, path string, index int, obj unstructured.Unstructured,
) *InvalidError {
	if obj.GetAnnotations() == nil ||
		len(obj.GetAnnotations()[manifestsv1alpha1.PackagePhaseAnnotation]) == 0 {
		return NewInvalidError(Violation{
			Reason: ViolationReasonMissingPhaseAnnotation,
			Location: &ViolationLocation{
				Path:          path,
				DocumentIndex: pointer.Int(index),
			},
		})
	}
	return nil
}

type ObjectGVKValidator struct{}

func (v *ObjectGVKValidator) Validate(ctx context.Context, packageContent *PackageContent) *InvalidError {
	return ValidateEachObject(ctx, packageContent.Manifests, v.validate)
}

func (*ObjectGVKValidator) validate(
	ctx context.Context, path string, index int, obj unstructured.Unstructured,
) *InvalidError {
	gvk := obj.GroupVersionKind()
	// Don't validate Group, because an empty group is valid and indicates the kube core API group.
	if len(gvk.Version) == 0 || len(gvk.Kind) == 0 {
		return NewInvalidError(Violation{
			Reason: ViolationReasonMissingGVK,
			Location: &ViolationLocation{
				Path:          path,
				DocumentIndex: pointer.Int(index),
			},
		})
	}
	return nil
}

type ObjectLabelsValidator struct{}

func (v *ObjectLabelsValidator) Validate(ctx context.Context, packageContent *PackageContent) *InvalidError {
	return ValidateEachObject(ctx, packageContent.Manifests, v.validate)
}

func (*ObjectLabelsValidator) validate(
	ctx context.Context, path string, index int, obj unstructured.Unstructured,
) *InvalidError {
	errList := validation.ValidateLabels(
		obj.GetLabels(), field.NewPath("metadata").Child("labels"))
	if len(errList) > 0 {
		return NewInvalidError(Violation{
			Reason:  ViolationReasonLabelsInvalid,
			Details: errList.ToAggregate().Error(),
			Location: &ViolationLocation{
				Path:          path,
				DocumentIndex: pointer.Int(index),
			},
		})
	}
	return nil
}

type PackageScopeValidator manifestsv1alpha1.PackageManifestScope

func (scope PackageScopeValidator) Validate(ctx context.Context, packageContent *PackageContent) *InvalidError {
	if !utils.Contains(
		packageContent.PackageManifest.Spec.Scopes,
		manifestsv1alpha1.PackageManifestScope(scope),
	) {
		// Package does not support installation in this scope.
		return NewInvalidError(Violation{
			Reason: ViolationReasonUnsupportedScope,
			Location: &ViolationLocation{
				Path: packages.PackageManifestFile,
			},
		})
	}
	return nil
}
