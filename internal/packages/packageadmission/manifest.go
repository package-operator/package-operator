package packageadmission

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/strings/slices"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func ValidatePackageManifest(ctx context.Context, scheme *runtime.Scheme, obj *manifestsv1alpha1.PackageManifest) (field.ErrorList, error) {
	allErrs := field.ErrorList{}

	if len(obj.Name) == 0 {
		allErrs = append(allErrs,
			field.Required(field.NewPath("metadata").Child("name"), ""))
	}

	spec := field.NewPath("spec")
	if len(obj.Spec.Scopes) == 0 {
		allErrs = append(allErrs,
			field.Required(spec.Child("scopes"), ""))
	}

	if len(obj.Spec.Phases) == 0 {
		allErrs = append(allErrs,
			field.Required(spec.Child("phases"), ""))
	}
	phaseNames := map[string]struct{}{}
	specPhases := spec.Child("phases")
	for i, phase := range obj.Spec.Phases {
		if _, alreadyExists := phaseNames[phase.Name]; alreadyExists {
			allErrs = append(allErrs,
				field.Invalid(specPhases.Index(i).Child("name"), phase.Name, "must be unique"))
		}
		phaseNames[phase.Name] = struct{}{}
	}

	specProbes := field.NewPath("spec").Child("availabilityProbes")
	if len(obj.Spec.AvailabilityProbes) == 0 {
		allErrs = append(allErrs,
			field.Required(specProbes, ""))
	}
	for i, probe := range obj.Spec.AvailabilityProbes {
		if len(probe.Probes) == 0 {
			allErrs = append(allErrs,
				field.Required(specProbes.Index(i).Child("probes"), ""))
		}
	}

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

		// TODO: check if this is a valid image tag (REPOSITORY[:TAG])
		if len(image.Image) < 1 {
			allErrs = append(allErrs,
				field.Invalid(specImages.Index(i).Child("image"), image.Image, "must be non empty"))
		}
	}

	configErrors := validatePackageManifestConfig(ctx, scheme, &obj.Spec.Config, spec.Child("config"))
	allErrs = append(allErrs, configErrors...)

	// Test config
	testTemplate := field.NewPath("test").Child("template")
	for i, template := range obj.Test.Template {
		el := validation.IsConfigMapKey(template.Name)
		if len(el) > 0 {
			allErrs = append(allErrs,
				field.Invalid(testTemplate.Index(i).Child("name"), template.Name, strings.Join(el, ", ")))
		}

		if len(configErrors) == 0 {
			configuration := map[string]interface{}{}
			if template.Context.Config != nil {
				if err := json.Unmarshal(template.Context.Config.Raw, &configuration); err != nil {
					return nil, fmt.Errorf("unmarshal config at test %s: %w", template.Name, err)
				}
			}

			valerrors, err := ValidatePackageConfiguration(
				ctx, scheme, &obj.Spec.Config, configuration, testTemplate.Index(i).Child("context").Child("config"))
			if err != nil {
				panic(err)
			}
			allErrs = append(allErrs, valerrors...)
		}
	}

	return allErrs, nil
}
