package packageadmission

import (
	"context"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/pruning"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

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

	return validatePackageConfigurationBySchema(ctx, nonVersionedSchema, configuration, fldPath)
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

	s, err := schema.NewStructural(nonVersionedSchema)
	if err != nil {
		return nil, err
	}

	// remove fields not part of the schema.
	pruning.Prune(configuration, s, true)

	// inject default values from schema.
	defaulting.Default(configuration, s)

	// validate configuration via schema.
	ferrs, err := validatePackageConfigurationBySchema(
		ctx, nonVersionedSchema, configuration, fldPath)
	if err != nil {
		return nil, err
	}
	return ferrs, nil
}
