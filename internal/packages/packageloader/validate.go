package packageloader

import (
	"context"

	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

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
	var errors []error
	for _, t := range l {
		if err := t.ValidatePackage(ctx, pkg); err != nil {
			errors = append(errors, err)
		}
	}
	return packages.NewInvalidAggregate(errors...)
}

func ValidateEachObject(ctx context.Context, pkg *packagecontent.Package, validate ValidateEachObjectFn) error {
	var errors []error
	for path, objects := range pkg.Objects {
		for i, object := range objects {
			if err := validate(ctx, path, i, object); err != nil {
				errors = append(errors, err)
			}
		}
	}
	return packages.NewInvalidAggregate(errors...)
}

func (v *ObjectPhaseAnnotationValidator) ValidatePackage(ctx context.Context, packageContent *packagecontent.Package) error {
	return ValidateEachObject(ctx, packageContent, v.validate)
}

func (*ObjectPhaseAnnotationValidator) validate(_ context.Context, path string, index int, obj unstructured.Unstructured) error {
	if obj.GetAnnotations() == nil ||
		len(obj.GetAnnotations()[manifestsv1alpha1.PackagePhaseAnnotation]) == 0 {
		return packages.NewInvalidError(packages.Violation{
			Reason: packages.ViolationReasonMissingPhaseAnnotation,
			Location: &packages.ViolationLocation{
				Path:          path,
				DocumentIndex: ptr.To(index),
			},
		})
	}
	return nil
}

// Objects with the same name/namespace/kind/group must only exist once over all phases.
// APIVersion does not matter for the check.
func (v *ObjectDuplicateValidator) ValidatePackage(_ context.Context, packageContent *packagecontent.Package) error {
	var errors []error
	visited := map[string]bool{}
	for path, objects := range packageContent.Objects {
		for i, object := range objects {
			gvk := object.GroupVersionKind()
			group := gvk.Group
			kind := gvk.Kind
			namespace := object.GetNamespace()
			name := object.GetName()
			key := group + "." + kind + "/" + namespace + "/" + name
			if _, ok := visited[key]; ok {
				errors = append(errors, packages.NewInvalidError(packages.Violation{
					Reason: packages.ViolationDuplicateObject,
					Location: &packages.ViolationLocation{
						Path:          path,
						DocumentIndex: ptr.To(i),
					},
				}))
			} else {
				visited[key] = true
			}
		}
	}
	return packages.NewInvalidAggregate(errors...)
}

func (v *ObjectGVKValidator) ValidatePackage(ctx context.Context, packageContent *packagecontent.Package) error {
	return ValidateEachObject(ctx, packageContent, v.validate)
}

func (*ObjectGVKValidator) validate(_ context.Context, path string, index int, obj unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	// Don't validate Group, because an empty group is valid and indicates the kube core API group.
	if len(gvk.Version) == 0 || len(gvk.Kind) == 0 {
		return packages.NewInvalidError(packages.Violation{
			Reason: packages.ViolationReasonMissingGVK,
			Location: &packages.ViolationLocation{
				Path:          path,
				DocumentIndex: ptr.To(index),
			},
		})
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
		return packages.NewInvalidError(packages.Violation{
			Reason:  packages.ViolationReasonLabelsInvalid,
			Details: errList.ToAggregate().Error(),
			Location: &packages.ViolationLocation{
				Path:          path,
				DocumentIndex: ptr.To(index),
			},
		})
	}
	return nil
}

func (scope PackageScopeValidator) ValidatePackage(_ context.Context, packageContent *packagecontent.Package) error {
	if !slices.Contains(packageContent.PackageManifest.Spec.Scopes, manifestsv1alpha1.PackageManifestScope(scope)) {
		// Package does not support installation in this scope.
		return packages.NewInvalidError(packages.Violation{
			Reason: packages.ViolationReasonUnsupportedScope,
			Location: &packages.ViolationLocation{
				Path: packages.PackageManifestFile,
			},
		})
	}

	return nil
}
