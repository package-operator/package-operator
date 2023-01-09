package validation

import (
	"context"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func ValidatePackageManifest(ctx context.Context, scheme *runtime.Scheme, obj *manifestsv1alpha1.PackageManifest) field.ErrorList {
	var allErrs field.ErrorList

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

	configErrors := ValidatePackageManifestConfig(ctx, scheme, &obj.Spec.Config, spec.Child("config"))
	allErrs = append(allErrs, configErrors...)

	// Test config
	testTemplate := field.NewPath("test").Child("template")
	for i, template := range obj.Test.Template {
		el := validation.IsConfigMapKey(template.Name)
		if len(el) > 0 {
			allErrs = append(allErrs,
				field.Invalid(testTemplate.Index(i).Child("name"), template.Name, allErrs.ToAggregate().Error()))
		}

		if len(configErrors) == 0 {
			valerrors, err := ValidatePackageConfiguration(
				ctx, scheme, &obj.Spec.Config, template.Context.Config, testTemplate.Index(i).Child("context").Child("config"))
			if err != nil {
				panic(err)
			}
			allErrs = append(allErrs, valerrors...)
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

func ValidatePackageManifestConfig(
	ctx context.Context, scheme *runtime.Scheme,
	config *manifestsv1alpha1.PackageManifestSpecConfig, fldPath *field.Path,
) field.ErrorList {
	if config.OpenAPIV3Schema == nil {
		return nil
	}

	var allErrs field.ErrorList
	schema := config.OpenAPIV3Schema
	if schema.Nullable {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("openAPIV3Schema.nullable"), "nullable cannot be true at the root"))
	}

	nonVersionedSchema := &apiextensions.JSONSchemaProps{}
	if err := scheme.Convert(config.OpenAPIV3Schema, nonVersionedSchema, nil); err != nil {
		panic(err)
	}

	opts := validationOptions{
		allowDefaults:                            true,
		requireRecognizedConversionReviewVersion: true,
		requireImmutableNames:                    false,
		requireOpenAPISchema:                     true,
		requireValidPropertyType:                 true,
		requireStructuralSchema:                  true,
		requirePrunedDefaults:                    true,
		requireAtomicSetType:                     true,
		requireMapListKeysMapSetValidation:       true,
	}
	allErrs = append(allErrs, validateCustomResourceDefinitionValidation(
		ctx, &apiextensions.CustomResourceValidation{
			OpenAPIV3Schema: nonVersionedSchema,
		}, false, opts, fldPath)...)
	return allErrs
}
