package validation

import (
	"context"
	"encoding/json"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extapivalidation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	kopenapivalidation "k8s.io/kube-openapi/pkg/validation/validate"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func ValidatePackageConfiguration(
	ctx context.Context, scheme *runtime.Scheme, mc *manifestsv1alpha1.PackageManifestSpecConfig,
	config *runtime.RawExtension, fldPath *field.Path,
) (field.ErrorList, error) {
	if mc.OpenAPIV3Schema == nil {
		return nil, nil
	}

	obj := map[string]interface{}{}
	if config != nil && len(config.Raw) > 0 {
		if err := json.Unmarshal(config.Raw, &obj); err != nil {
			return nil, err
		}
	}

	nonVersionedSchema := &apiextensions.JSONSchemaProps{}
	if err := scheme.Convert(mc.OpenAPIV3Schema, nonVersionedSchema, nil); err != nil {
		return nil, err
	}

	return ValidatePackageConfigurationBySchema(ctx, scheme, nonVersionedSchema, obj, fldPath)
}

func ValidatePackageConfigurationBySchema(
	ctx context.Context, scheme *runtime.Scheme, schema *apiextensions.JSONSchemaProps,
	config map[string]interface{}, fldPath *field.Path,
) (field.ErrorList, error) {
	if schema == nil {
		return nil, nil
	}

	openapiSchema := &spec.Schema{}
	if err := extapivalidation.ConvertJSONSchemaProps(schema, openapiSchema); err != nil {
		return nil, err
	}

	v := kopenapivalidation.NewSchemaValidator(openapiSchema, nil, "", strfmt.Default)
	return extapivalidation.ValidateCustomResource(fldPath, config, v), nil
}
