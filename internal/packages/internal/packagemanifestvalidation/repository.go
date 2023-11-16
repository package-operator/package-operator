package packagemanifestvalidation

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"package-operator.run/internal/apis/manifests"
)

func ValidateRepositoryEntry(_ context.Context, obj *manifests.RepositoryEntry) (field.ErrorList, error) {
	allErrs := field.ErrorList{}
	if len(obj.Name) == 0 {
		allErrs = append(allErrs,
			field.Required(field.NewPath("metadata").Child("name"), ""))
	}

	dataPath := field.NewPath("data")
	if len(obj.Data.Image) == 0 {
		allErrs = append(allErrs,
			field.Required(dataPath.Child("image"), ""))
	}
	if len(obj.Data.Digest) == 0 {
		allErrs = append(allErrs,
			field.Required(dataPath.Child("digest"), ""))
	}
	if len(obj.Data.Versions) == 0 {
		allErrs = append(allErrs,
			field.Required(dataPath.Child("versions"), ""))
	}

	// Constraints
	allErrs = append(allErrs, validateConstraints(
		dataPath.Child("constraints"), obj.Data.Constraints)...)

	return allErrs, nil
}
