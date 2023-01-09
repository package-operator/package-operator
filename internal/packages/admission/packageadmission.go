package admission

import (
	"context"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/pruning"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages/admission/validation"
)

type packageManifestValidator func(context.Context, *runtime.Scheme, *manifestsv1alpha1.PackageManifest) field.ErrorList

type PackageAdmissionController struct {
	scheme *runtime.Scheme
	packageManifestValidator
}

func NewPackageAdmissionController(scheme *runtime.Scheme) *PackageAdmissionController {
	return &PackageAdmissionController{
		scheme:                   scheme,
		packageManifestValidator: validation.ValidatePackageManifest,
	}
}

// Validates PackageManifest and prunes,defaults and validates the given configuration.
func (pac *PackageAdmissionController) Admit(
	ctx context.Context, configuration map[string]interface{},
	manifest *manifestsv1alpha1.PackageManifest,
) (err error) {

	manifestFieldErrors := pac.packageManifestValidator(ctx, pac.scheme, manifest)
	if len(manifestFieldErrors) > 0 {
		return manifestFieldErrors.ToAggregate()
	}

	if manifest.Spec.Config.OpenAPIV3Schema == nil {
		// No Schema -> Everything is pruned.
		for k := range configuration {
			delete(configuration, k)
		}
		return nil
	}

	nonVersionedSchema := &apiextensions.JSONSchemaProps{}
	if err := pac.scheme.Convert(
		manifest.Spec.Config.OpenAPIV3Schema, nonVersionedSchema, nil,
	); err != nil {
		return err
	}

	s, err := extschema.NewStructural(nonVersionedSchema)
	if err != nil {
		return err
	}

	// remove fields not part of the schema.
	pruning.Prune(configuration, s, true)

	// inject default values from schema.
	defaulting.Default(configuration, s)

	// validate configuration via schema.
	ferrs, err := validation.ValidatePackageConfigurationBySchema(
		ctx, pac.scheme, nonVersionedSchema, configuration, nil)
	if err != nil {
		return err
	}
	return ferrs.ToAggregate()
}
