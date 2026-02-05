package packagemanifestvalidation

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"pkg.package-operator.run/semver"

	"package-operator.run/internal/apis/manifests"
)

// Validates the PackageManifest.
func ValidatePackageManifest(ctx context.Context, obj *manifests.PackageManifest) (field.ErrorList, error) {
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

	// Constraints
	allErrs = append(allErrs, validateConstraints(
		field.NewPath("spec").Child("constraints"), obj.Spec.Constraints)...)

	configErrors := validatePackageManifestConfig(ctx, &obj.Spec.Config, spec.Child("config"))
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
			configuration := map[string]any{}
			if template.Context.Config != nil {
				if err := json.Unmarshal(template.Context.Config.Raw, &configuration); err != nil {
					return nil, fmt.Errorf("unmarshal config at test %s: %w", template.Name, err)
				}
			}

			valerrors, err := ValidatePackageConfiguration(
				ctx, &obj.Spec.Config, configuration, testTemplate.Index(i).Child("context").Child("config"))
			if err != nil {
				panic(err)
			}
			allErrs = append(allErrs, valerrors...)
		}
	}
	if obj.Test.Kubeconform != nil {
		if len(obj.Test.Kubeconform.KubernetesVersion) == 0 {
			allErrs = append(allErrs,
				field.Required(field.NewPath("test").Child("kubeconform").Child("kubernetesVersion"), ""))
		}
	}

	return allErrs, nil
}

func validateConstraints(path *field.Path, constraints []manifests.PackageManifestConstraint) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, constraint := range constraints {
		cpath := path.Index(i)
		if constraint.PlatformVersion != nil {
			cpath = cpath.Child("platformVersion")
			if len(constraint.PlatformVersion.Name) == 0 {
				allErrs = append(allErrs,
					field.Required(cpath.Child("name"), ""))
			}
			if len(constraint.PlatformVersion.Range) == 0 {
				allErrs = append(allErrs,
					field.Required(cpath.Child("range"), ""))
			} else if _, cerr := semver.NewConstraint(constraint.PlatformVersion.Range); cerr != nil {
				allErrs = append(allErrs,
					field.Invalid(cpath.Child("range"), constraint.PlatformVersion.Range, "improper constraint"))
			}
		}
	}

	return allErrs
}
