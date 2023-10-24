package packagevalidation

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/packagetypes"
)

// DefaultObjectValidators is a list of object validators that should be executed as a minimum standard.
var DefaultObjectValidators = ObjectValidatorList{
	&ObjectDuplicateValidator{}, &ObjectGVKValidator{},
	&ObjectLabelsValidator{}, &ObjectPhaseAnnotationValidator{},
}

// ObjectValidatorList runs a list of validators and joins all errors.
type ObjectValidatorList []packagetypes.ObjectValidator

func (l ObjectValidatorList) ValidateObjects(
	ctx context.Context,
	manifest *manifests.PackageManifest,
	objects map[string][]unstructured.Unstructured,
) error {
	var errs []error
	for _, t := range l {
		if err := t.ValidateObjects(ctx, manifest, objects); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Function given to ValidateEachObject to validate individual objects in a package.
type ValidateEachObjectFn func(
	ctx context.Context, path string, index int,
	obj unstructured.Unstructured, manifest *manifests.PackageManifest,
) error

// ValidateEachObject iterates over each object in a package and runs the given validation function.
func ValidateEachObject(
	ctx context.Context,
	manifest *manifests.PackageManifest,
	objects map[string][]unstructured.Unstructured,
	validate ValidateEachObjectFn,
) error {
	var errs []error
	for path, objects := range objects {
		for i, object := range objects {
			if err := validate(ctx, path, i, object, manifest); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// Validates that the PKO phase-annotation is set on all objects.
type ObjectPhaseAnnotationValidator struct{}

var _ packagetypes.ObjectValidator = (*ObjectPhaseAnnotationValidator)(nil)

func (v *ObjectPhaseAnnotationValidator) ValidateObjects(
	ctx context.Context,
	manifest *manifests.PackageManifest,
	objects map[string][]unstructured.Unstructured,
) error {
	return ValidateEachObject(ctx, manifest, objects, v.validate)
}

func (*ObjectPhaseAnnotationValidator) validate(
	_ context.Context, path string, index int,
	obj unstructured.Unstructured, _ *manifests.PackageManifest,
) error {
	if obj.GetAnnotations() == nil ||
		len(obj.GetAnnotations()[manifests.PackagePhaseAnnotation]) == 0 {
		return packagetypes.ViolationError{
			Reason: packagetypes.ViolationReasonMissingPhaseAnnotation,
			Path:   path,
			Index:  ptr.To(index),
		}
	}
	return nil
}

// Validates that Objects with the same name/namespace/kind/group must only exist once over all phases.
// APIVersion does not matter for the check.
type ObjectDuplicateValidator struct{}

var _ packagetypes.ObjectValidator = (*ObjectDuplicateValidator)(nil)

func (v *ObjectDuplicateValidator) ValidateObjects(
	_ context.Context,
	_ *manifests.PackageManifest,
	objects map[string][]unstructured.Unstructured,
) error {
	var errs []error
	visited := map[string]bool{}
	for path, objects := range objects {
		for idx, object := range objects {
			gvk := object.GroupVersionKind()
			groupKind := gvk.GroupKind().String()
			object := object
			objectKey := client.ObjectKeyFromObject(&object).String() // namespace and name
			key := fmt.Sprintf("%s %s", groupKind, objectKey)
			if _, ok := visited[key]; ok {
				errs = append(errs, packagetypes.ViolationError{
					Reason: packagetypes.ViolationReasonDuplicateObject,
					Path:   path,
					Index:  ptr.To(idx),
				})
			} else {
				visited[key] = true
			}
		}
	}

	return errors.Join(errs...)
}

// Validates that every object has Group, Version and Kind set.
// e.g. apiVersion: and kind:.
type ObjectGVKValidator struct{}

var _ packagetypes.ObjectValidator = (*ObjectGVKValidator)(nil)

func (v *ObjectGVKValidator) ValidateObjects(
	ctx context.Context,
	manifest *manifests.PackageManifest,
	objects map[string][]unstructured.Unstructured,
) error {
	return ValidateEachObject(ctx, manifest, objects, v.validate)
}

func (*ObjectGVKValidator) validate(
	_ context.Context, path string, index int,
	obj unstructured.Unstructured, _ *manifests.PackageManifest,
) error {
	gvk := obj.GroupVersionKind()
	// Don't validate Group, because an empty group is valid and indicates the kube core API group.
	if len(gvk.Version) == 0 || len(gvk.Kind) == 0 {
		return packagetypes.ViolationError{
			Reason: packagetypes.ViolationReasonMissingGVK,
			Path:   path,
			Index:  ptr.To(index),
		}
	}
	return nil
}

// Validates that all labels are valid.
type ObjectLabelsValidator struct{}

var _ packagetypes.ObjectValidator = (*ObjectLabelsValidator)(nil)

func (v *ObjectLabelsValidator) ValidateObjects(
	ctx context.Context,
	manifest *manifests.PackageManifest,
	objects map[string][]unstructured.Unstructured,
) error {
	return ValidateEachObject(ctx, manifest, objects, v.validate)
}

func (*ObjectLabelsValidator) validate(
	_ context.Context, path string, index int,
	obj unstructured.Unstructured, _ *manifests.PackageManifest,
) error {
	errList := validation.ValidateLabels(
		obj.GetLabels(), field.NewPath("metadata").Child("labels"))
	if len(errList) > 0 {
		return packagetypes.ViolationError{
			Reason:  packagetypes.ViolationReasonLabelsInvalid,
			Details: errList.ToAggregate().Error(),
			Path:    path,
			Index:   ptr.To(index),
		}
	}
	return nil
}
