package packagemanifestvalidation

import (
	"context"

	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/pruning"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"package-operator.run/internal/apis/manifests"
)

// Validates configuration against the PackageManifests OpenAPISchema.
func ValidatePackageConfiguration(
	ctx context.Context, mc *manifests.PackageManifestSpecConfig,
	configuration map[string]interface{}, fldPath *field.Path,
) (field.ErrorList, error) {
	if mc.OpenAPIV3Schema == nil {
		return nil, nil
	}

	return validatePackageConfigurationBySchema(ctx, mc.OpenAPIV3Schema, configuration, fldPath)
}

// Prunes, Defaults and Validates configuration against the PackageManifests OpenAPISchema so it's ready to be used.
func AdmitPackageConfiguration(
	ctx context.Context, configuration map[string]interface{},
	manifest *manifests.PackageManifest, fldPath *field.Path,
) (field.ErrorList, error) {
	if manifest.Spec.Config.OpenAPIV3Schema == nil {
		// Prune all configuration fields
		for k := range configuration {
			delete(configuration, k)
		}
		return nil, nil
	}

	s, err := schema.NewStructural(manifest.Spec.Config.OpenAPIV3Schema)
	if err != nil {
		return nil, err
	}

	// remove fields not part of the schema.
	pruning.Prune(configuration, s, true)

	// inject default values from schema.
	defaulting.Default(configuration, s)

	// validate configuration via schema.
	ferrs, err := validatePackageConfigurationBySchema(
		ctx, manifest.Spec.Config.OpenAPIV3Schema, configuration, fldPath)
	if err != nil {
		return nil, err
	}
	return ferrs, nil
}
