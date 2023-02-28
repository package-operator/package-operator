package packageadmission

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/strings/slices"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func ValidatePackageManifestLock(_ context.Context, obj *manifestsv1alpha1.PackageManifestLock) (field.ErrorList, error) {
	var allErrs field.ErrorList

	specImages := field.NewPath("spec").Child("images")
	existingNames := []string{}
	for i, image := range obj.Spec.Images {
		switch {
		case len(image.Name) < 1:
			allErrs = append(allErrs,
				field.Invalid(specImages.Index(i).Child("name"), image.Name, "must be non empty"))
		case slices.Contains(existingNames, image.Name):
			allErrs = append(allErrs,
				field.Invalid(specImages.Index(i).Child("name"), image.Name, "must be unique"))
		default:
			existingNames = append(existingNames, image.Name)
		}

		if len(image.Image) < 1 {
			allErrs = append(allErrs,
				field.Invalid(specImages.Index(i).Child("image"), image.Image, "must be non empty"))
		}

		if len(image.Digest) < 1 {
			allErrs = append(allErrs,
				field.Invalid(specImages.Index(i).Child("digest"), image.Digest, "must be non empty"))
		}
	}

	return allErrs, nil
}
