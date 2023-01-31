package packageadmission

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/pruning"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func ValidatePackageManifest(ctx context.Context, scheme *runtime.Scheme, obj *manifestsv1alpha1.PackageManifest) (field.ErrorList, error) {
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

	configErrors := validatePackageManifestConfig(ctx, scheme, &obj.Spec.Config, spec.Child("config"))
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

func ValidatePackageConfiguration(
	ctx context.Context, scheme *runtime.Scheme, mc *manifestsv1alpha1.PackageManifestSpecConfig,
	configuration map[string]interface{}, fldPath *field.Path,
) (field.ErrorList, error) {
	if mc.OpenAPIV3Schema == nil {
		return nil, nil
	}

	nonVersionedSchema := &apiextensions.JSONSchemaProps{}
	if err := scheme.Convert(mc.OpenAPIV3Schema, nonVersionedSchema, nil); err != nil {
		return nil, err
	}

	return validatePackageConfigurationBySchema(ctx, scheme, nonVersionedSchema, configuration, fldPath)
}

func AdmitPackageConfiguration(
	ctx context.Context, scheme *runtime.Scheme, configuration map[string]interface{},
	manifest *manifestsv1alpha1.PackageManifest, fldPath *field.Path,
) (field.ErrorList, error) {
	if manifest.Spec.Config.OpenAPIV3Schema == nil {
		// Prune all configuration fields
		for k := range configuration {
			delete(configuration, k)
		}
		return nil, nil
	}
	nonVersionedSchema := &apiextensions.JSONSchemaProps{}
	if err := scheme.Convert(
		manifest.Spec.Config.OpenAPIV3Schema, nonVersionedSchema, nil,
	); err != nil {
		return nil, err
	}

	s, err := extschema.NewStructural(nonVersionedSchema)
	if err != nil {
		return nil, err
	}

	// remove fields not part of the schema.
	pruning.Prune(configuration, s, true)

	// inject default values from schema.
	defaulting.Default(configuration, s)

	// validate configuration via schema.
	ferrs, err := validatePackageConfigurationBySchema(
		ctx, scheme, nonVersionedSchema, configuration, fldPath)
	if err != nil {
		return nil, err
	}
	return ferrs, nil
}
