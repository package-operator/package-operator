package packageloader

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages"
	"package-operator.run/internal/packages/packagecontent"
)

type (

	// Runs a list of Validator over the given content.
	ValidatorList                  []Validator
	ValidateEachObjectFn           func(ctx context.Context, path string, index int, obj unstructured.Unstructured) error
	ObjectPhaseAnnotationValidator struct{}
	ObjectDuplicateValidator       struct{}
	ObjectGVKValidator             struct{}
	ObjectLabelsValidator          struct{}
	PackageScopeValidator          manifestsv1alpha1.PackageManifestScope
)

var (
	_ Validator = (ValidatorList)(nil)
	_ Validator = (*ObjectPhaseAnnotationValidator)(nil)
	_ Validator = (*ObjectDuplicateValidator)(nil)
	_ Validator = (*ObjectGVKValidator)(nil)
	_ Validator = (*ObjectLabelsValidator)(nil)
	_ Validator = (PackageScopeValidator)("")

	DefaultValidators = ValidatorList{
		&ObjectDuplicateValidator{}, &ObjectGVKValidator{},
		&ObjectLabelsValidator{}, &ObjectPhaseAnnotationValidator{},
	}
)

func (l ValidatorList) ValidatePackage(ctx context.Context, pkg *packagecontent.Package) error {
	var errs []error
	for _, t := range l {
		if err := t.ValidatePackage(ctx, pkg); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func ValidateEachObject(ctx context.Context, pkg *packagecontent.Package, validate ValidateEachObjectFn) error {
	var errs []error
	for path, objects := range pkg.Objects {
		for i, object := range objects {
			if err := validate(ctx, path, i, object); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (v *ObjectPhaseAnnotationValidator) ValidatePackage(ctx context.Context, packageContent *packagecontent.Package) error {
	return ValidateEachObject(ctx, packageContent, v.validate)
}

func (*ObjectPhaseAnnotationValidator) validate(_ context.Context, path string, index int, obj unstructured.Unstructured) error {
	if obj.GetAnnotations() == nil ||
		len(obj.GetAnnotations()[manifestsv1alpha1.PackagePhaseAnnotation]) == 0 {
		return packages.ViolationError{
			Reason: packages.ViolationReasonMissingPhaseAnnotation,
			Path:   path,
			Index:  packages.Index(index),
		}
	}
	return nil
}

// Objects with the same name/namespace/kind/group must only exist once over all phases.
// APIVersion does not matter for the check.
func (v *ObjectDuplicateValidator) ValidatePackage(_ context.Context, packageContent *packagecontent.Package) error {
	var errs []error
	visited := map[string]bool{}
	for path, objects := range packageContent.Objects {
		for idx, object := range objects {
			gvk := object.GroupVersionKind()
			groupKind := gvk.GroupKind().String()
			object := object
			objectKey := client.ObjectKeyFromObject(&object).String() // namespace and name
			key := fmt.Sprintf("%s %s", groupKind, objectKey)
			if _, ok := visited[key]; ok {
				errs = append(errs, packages.ViolationError{
					Reason: packages.ViolationReasonDuplicateObject,
					Path:   path,
					Index:  packages.Index(idx),
				})
			} else {
				visited[key] = true
			}
		}
	}

	return errors.Join(errs...)
}

func (v *ObjectGVKValidator) ValidatePackage(ctx context.Context, packageContent *packagecontent.Package) error {
	return ValidateEachObject(ctx, packageContent, v.validate)
}

func (*ObjectGVKValidator) validate(_ context.Context, path string, index int, obj unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	// Don't validate Group, because an empty group is valid and indicates the kube core API group.
	if len(gvk.Version) == 0 || len(gvk.Kind) == 0 {
		return packages.ViolationError{
			Reason: packages.ViolationReasonMissingGVK,
			Path:   path,
			Index:  packages.Index(index),
		}
	}
	return nil
}

func (v *ObjectLabelsValidator) ValidatePackage(ctx context.Context, packageContent *packagecontent.Package) error {
	return ValidateEachObject(ctx, packageContent, v.validate)
}

func (*ObjectLabelsValidator) validate(_ context.Context, path string, index int, obj unstructured.Unstructured) error {
	errList := validation.ValidateLabels(
		obj.GetLabels(), field.NewPath("metadata").Child("labels"))
	if len(errList) > 0 {
		return packages.ViolationError{
			Reason:  packages.ViolationReasonLabelsInvalid,
			Details: errList.ToAggregate().Error(),
			Path:    path,
			Index:   packages.Index(index),
		}
	}
	return nil
}

func (scope PackageScopeValidator) ValidatePackage(_ context.Context, packageContent *packagecontent.Package) error {
	if !slices.Contains(packageContent.PackageManifest.Spec.Scopes, manifestsv1alpha1.PackageManifestScope(scope)) {
		// Package does not support installation in this scope.
		return packages.ViolationError{
			Reason: packages.ViolationReasonUnsupportedScope,
			Path:   packages.PackageManifestFilename,
		}
	}

	return nil
}
