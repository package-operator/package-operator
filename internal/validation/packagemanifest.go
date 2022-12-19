package validation

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsvalidation "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	structuraldefaulting "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	apiservervalidation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiservercel "k8s.io/apiserver/pkg/cel"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

var (
	// printerColumnDatatypes                = sets.NewString("integer", "number", "string", "boolean", "date")
	// customResourceColumnDefinitionFormats = sets.NewString("int32", "int64", "float", "double", "byte", "date", "date-time", "password")
	openapiV3Types = sets.NewString("string", "number", "integer", "boolean", "array", "object")
	// unbounded uses nil to represent an unbounded cardinality value.
	unbounded *uint64 = nil
)

const (
	// StaticEstimatedCostLimit represents the largest-allowed static CEL cost on a per-expression basis.
	StaticEstimatedCostLimit = 10000000
	// StaticEstimatedCRDCostLimit represents the largest-allowed total cost for the x-kubernetes-validations rules of a CRD.
	StaticEstimatedCRDCostLimit = 100000000
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

	testTemplate := field.NewPath("test").Child("template")
	for i, template := range obj.Test.Template {
		el := validation.IsConfigMapKey(template.Name)
		if len(el) > 0 {
			allErrs = append(allErrs,
				field.Invalid(testTemplate.Index(i).Child("name"), template.Name, allErrs.ToAggregate().Error()))
		}
	}

	allErrs = append(allErrs, ValidatePackageManifestConfig(ctx, scheme, &obj.Spec.Config, spec.Child("config"))...)

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

// validationOptions groups several validation options, to avoid passing multiple bool parameters to methods
type validationOptions struct {
	// allowDefaults permits the validation schema to contain default attributes
	allowDefaults bool
	// disallowDefaultsReason gives a reason as to why allowDefaults is false (for better user feedback)
	disallowDefaultsReason string
	// requireRecognizedConversionReviewVersion requires accepted webhook conversion versions to contain a recognized version
	requireRecognizedConversionReviewVersion bool
	// requireImmutableNames disables changing spec.names
	requireImmutableNames bool
	// requireOpenAPISchema requires an openapi V3 schema be specified
	requireOpenAPISchema bool
	// requireValidPropertyType requires property types specified in the validation schema to be valid openapi v3 types
	requireValidPropertyType bool
	// requireStructuralSchema indicates that any schemas present must be structural
	requireStructuralSchema bool
	// requirePrunedDefaults indicates that defaults must be pruned
	requirePrunedDefaults bool
	// requireAtomicSetType indicates that the items type for a x-kubernetes-list-type=set list must be atomic.
	requireAtomicSetType bool
	// requireMapListKeysMapSetValidation indicates that:
	// 1. For x-kubernetes-list-type=map list, key fields are not nullable, and are required or have a default
	// 2. For x-kubernetes-list-type=map or x-kubernetes-list-type=set list, the whole item must not be nullable.
	requireMapListKeysMapSetValidation bool
}

// specStandardValidator applies validations for different OpenAPI specification versions.
type specStandardValidator interface {
	validate(spec *apiextensions.JSONSchemaProps, fldPath *field.Path) field.ErrorList
	withForbiddenDefaults(reason string) specStandardValidator

	// insideResourceMeta returns true when validating either TypeMeta or ObjectMeta, from an embedded resource or on the top-level.
	insideResourceMeta() bool
	withInsideResourceMeta() specStandardValidator

	// forbidOldSelfValidations returns the path to the first ancestor of the visited path that can't be safely correlated between two revisions of an object, or nil if there is no such path
	forbidOldSelfValidations() *field.Path
	withForbidOldSelfValidations(path *field.Path) specStandardValidator
}

// validateCustomResourceDefinitionValidation statically validates
// context is passed for supporting context cancellation during cel validation when validating defaults
func validateCustomResourceDefinitionValidation(ctx context.Context, customResourceValidation *apiextensions.CustomResourceValidation, statusSubresourceEnabled bool, opts validationOptions, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if customResourceValidation == nil {
		return allErrs
	}

	if schema := customResourceValidation.OpenAPIV3Schema; schema != nil {
		// if the status subresource is enabled, only certain fields are allowed inside the root schema.
		// these fields are chosen such that, if status is extracted as properties["status"], it's validation is not lost.
		if statusSubresourceEnabled {
			v := reflect.ValueOf(schema).Elem()
			for i := 0; i < v.NumField(); i++ {
				// skip zero values
				if value := v.Field(i).Interface(); reflect.DeepEqual(value, reflect.Zero(reflect.TypeOf(value)).Interface()) {
					continue
				}

				fieldName := v.Type().Field(i).Name

				// only "object" type is valid at root of the schema since validation schema for status is extracted as properties["status"]
				if fieldName == "Type" {
					if schema.Type != "object" {
						allErrs = append(allErrs, field.Invalid(fldPath.Child("openAPIV3Schema.type"), schema.Type, `only "object" is allowed as the type at the root of the schema if the status subresource is enabled`))
						break
					}
					continue
				}

				if !allowedAtRootSchema(fieldName) {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("openAPIV3Schema"), *schema, fmt.Sprintf(`only %v fields are allowed at the root of the schema if the status subresource is enabled`, allowedFieldsAtRootSchema)))
					break
				}
			}
		}

		if schema.Nullable {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("openAPIV3Schema.nullable"), "nullable cannot be true at the root"))
		}

		openAPIV3Schema := &specStandardValidatorV3{
			allowDefaults:            opts.allowDefaults,
			disallowDefaultsReason:   opts.disallowDefaultsReason,
			requireValidPropertyType: opts.requireValidPropertyType,
		}

		var celContext *apiextensionsvalidation.CELSchemaContext
		var structuralSchemaInitErrs field.ErrorList
		if opts.requireStructuralSchema {
			if ss, err := structuralschema.NewStructural(schema); err != nil {
				// These validation errors overlap with  OpenAPISchema validation errors so we keep track of them
				// separately and only show them if OpenAPISchema validation does not report any errors.
				structuralSchemaInitErrs = append(structuralSchemaInitErrs, field.Invalid(fldPath.Child("openAPIV3Schema"), "", err.Error()))
			} else if validationErrors := structuralschema.ValidateStructural(fldPath.Child("openAPIV3Schema"), ss); len(validationErrors) > 0 {
				allErrs = append(allErrs, validationErrors...)
			} else if validationErrors, err := structuraldefaulting.ValidateDefaults(ctx, fldPath.Child("openAPIV3Schema"), ss, true, opts.requirePrunedDefaults); err != nil {
				// this should never happen
				allErrs = append(allErrs, field.Invalid(fldPath.Child("openAPIV3Schema"), "", err.Error()))
			} else if len(validationErrors) > 0 {
				allErrs = append(allErrs, validationErrors...)
			} else {
				// Only initialize CEL rule validation context if the structural schemas are valid.
				// A nil CELSchemaContext indicates that no CEL validation should be attempted.
				celContext = apiextensionsvalidation.RootCELContext(schema)
			}
		}
		allErrs = append(allErrs, ValidateCustomResourceDefinitionOpenAPISchema(schema, fldPath.Child("openAPIV3Schema"), openAPIV3Schema, true, &opts, celContext).AllErrors()...)

		if len(allErrs) == 0 && len(structuralSchemaInitErrs) > 0 {
			// Structural schema initialization errors overlap with OpenAPISchema validation errors so we only show them
			// if there are no OpenAPISchema validation errors.
			allErrs = append(allErrs, structuralSchemaInitErrs...)
		}

		if celContext != nil && celContext.TotalCost != nil {
			if celContext.TotalCost.Total > StaticEstimatedCRDCostLimit {
				for _, expensive := range celContext.TotalCost.MostExpensive {
					costErrorMsg := "contributed to estimated rule cost total exceeding cost limit for entire OpenAPIv3 schema"
					allErrs = append(allErrs, field.Forbidden(expensive.Path, costErrorMsg))
				}

				costErrorMsg := getCostErrorMessage("x-kubernetes-validations estimated rule cost total for entire OpenAPIv3 schema", celContext.TotalCost.Total, StaticEstimatedCRDCostLimit)
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("openAPIV3Schema"), costErrorMsg))
			}
		}
	}

	// if validation passed otherwise, make sure we can actually construct a schema validator from this custom resource validation.
	if len(allErrs) == 0 {
		if _, _, err := apiservervalidation.NewSchemaValidator(customResourceValidation); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, "", fmt.Sprintf("error building validator: %v", err)))
		}
	}
	return allErrs
}

var metaFields = sets.NewString("metadata", "kind", "apiVersion")

// OpenAPISchemaErrorList tracks all validation errors reported ValidateCustomResourceDefinitionOpenAPISchema
// with CEL related errors kept separate from schema related errors.
type OpenAPISchemaErrorList struct {
	SchemaErrors field.ErrorList
	CELErrors    field.ErrorList
}

// AppendErrors appends all errors in the provided list with the errors of this list.
func (o *OpenAPISchemaErrorList) AppendErrors(list *OpenAPISchemaErrorList) {
	if o == nil || list == nil {
		return
	}
	o.SchemaErrors = append(o.SchemaErrors, list.SchemaErrors...)
	o.CELErrors = append(o.CELErrors, list.CELErrors...)
}

// AllErrors returns a list containing both schema and CEL errors.
func (o *OpenAPISchemaErrorList) AllErrors() field.ErrorList {
	if o == nil {
		return field.ErrorList{}
	}
	return append(o.SchemaErrors, o.CELErrors...)
}

// ValidateCustomResourceDefinitionOpenAPISchema statically validates
func ValidateCustomResourceDefinitionOpenAPISchema(schema *apiextensions.JSONSchemaProps, fldPath *field.Path, ssv specStandardValidator, isRoot bool, opts *validationOptions, celContext *apiextensionsvalidation.CELSchemaContext) *OpenAPISchemaErrorList {
	allErrs := &OpenAPISchemaErrorList{SchemaErrors: field.ErrorList{}, CELErrors: field.ErrorList{}}

	if schema == nil {
		return allErrs
	}
	allErrs.SchemaErrors = append(allErrs.SchemaErrors, ssv.validate(schema, fldPath)...)

	if schema.UniqueItems {
		allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Forbidden(fldPath.Child("uniqueItems"), "uniqueItems cannot be set to true since the runtime complexity becomes quadratic"))
	}

	// additionalProperties and properties are mutual exclusive because otherwise they
	// contradict Kubernetes' API convention to ignore unknown fields.
	//
	// In other words:
	// - properties are for structs,
	// - additionalProperties are for map[string]interface{}
	//
	// Note: when patternProperties is added to OpenAPI some day, this will have to be
	//       restricted like additionalProperties.
	if schema.AdditionalProperties != nil {
		if len(schema.Properties) != 0 {
			if !schema.AdditionalProperties.Allows || schema.AdditionalProperties.Schema != nil {
				allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Forbidden(fldPath.Child("additionalProperties"), "additionalProperties and properties are mutual exclusive"))
			}
		}
		// Note: we forbid additionalProperties at resource root, both embedded and top-level.
		//       But further inside, additionalProperites is possible, e.g. for labels or annotations.
		subSsv := ssv
		if ssv.insideResourceMeta() {
			// we have to forbid defaults inside additionalProperties because pruning without actual value is ambiguous
			subSsv = ssv.withForbiddenDefaults("inside additionalProperties applying to object metadata")
		}
		allErrs.AppendErrors(ValidateCustomResourceDefinitionOpenAPISchema(schema.AdditionalProperties.Schema, fldPath.Child("additionalProperties"), subSsv, false, opts, celContext.ChildAdditionalPropertiesContext(schema.AdditionalProperties.Schema)))
	}

	if len(schema.Properties) != 0 {
		for property, jsonSchema := range schema.Properties {
			subSsv := ssv

			if !cel.MapIsCorrelatable(schema.XMapType) {
				subSsv = subSsv.withForbidOldSelfValidations(fldPath)
			}

			if (isRoot || schema.XEmbeddedResource) && metaFields.Has(property) {
				// we recurse into the schema that applies to ObjectMeta.
				subSsv = subSsv.withInsideResourceMeta()
				if isRoot {
					subSsv = subSsv.withForbiddenDefaults(fmt.Sprintf("in top-level %s", property))
				}
			}
			propertySchema := jsonSchema
			allErrs.AppendErrors(ValidateCustomResourceDefinitionOpenAPISchema(&propertySchema, fldPath.Child("properties").Key(property), subSsv, false, opts, celContext.ChildPropertyContext(&propertySchema, property)))
		}
	}

	allErrs.AppendErrors(ValidateCustomResourceDefinitionOpenAPISchema(schema.Not, fldPath.Child("not"), ssv, false, opts, nil))

	if len(schema.AllOf) != 0 {
		for i, jsonSchema := range schema.AllOf {
			allOfSchema := jsonSchema
			allErrs.AppendErrors(ValidateCustomResourceDefinitionOpenAPISchema(&allOfSchema, fldPath.Child("allOf").Index(i), ssv, false, opts, nil))
		}
	}

	if len(schema.OneOf) != 0 {
		for i, jsonSchema := range schema.OneOf {
			oneOfSchema := jsonSchema
			allErrs.AppendErrors(ValidateCustomResourceDefinitionOpenAPISchema(&oneOfSchema, fldPath.Child("oneOf").Index(i), ssv, false, opts, nil))
		}
	}

	if len(schema.AnyOf) != 0 {
		for i, jsonSchema := range schema.AnyOf {
			anyOfSchema := jsonSchema
			allErrs.AppendErrors(ValidateCustomResourceDefinitionOpenAPISchema(&anyOfSchema, fldPath.Child("anyOf").Index(i), ssv, false, opts, nil))
		}
	}

	if len(schema.Definitions) != 0 {
		for definition, jsonSchema := range schema.Definitions {
			definitionSchema := jsonSchema
			allErrs.AppendErrors(ValidateCustomResourceDefinitionOpenAPISchema(&definitionSchema, fldPath.Child("definitions").Key(definition), ssv, false, opts, nil))
		}
	}

	if schema.Items != nil {
		subSsv := ssv

		// we can only correlate old/new items for "map" and "set" lists, and correlation of
		// "set" elements by identity is not supported for cel (x-kubernetes-validations)
		// rules. an unset list type defaults to "atomic".
		if schema.XListType == nil || *schema.XListType != "map" {
			subSsv = subSsv.withForbidOldSelfValidations(fldPath)
		}

		allErrs.AppendErrors(ValidateCustomResourceDefinitionOpenAPISchema(schema.Items.Schema, fldPath.Child("items"), subSsv, false, opts, celContext.ChildItemsContext(schema.Items.Schema)))
		if len(schema.Items.JSONSchemas) != 0 {
			for i, jsonSchema := range schema.Items.JSONSchemas {
				itemsSchema := jsonSchema
				allErrs.AppendErrors(ValidateCustomResourceDefinitionOpenAPISchema(&itemsSchema, fldPath.Child("items").Index(i), subSsv, false, opts, celContext.ChildItemsContext(&itemsSchema)))
			}
		}
	}

	if schema.Dependencies != nil {
		for dependency, jsonSchemaPropsOrStringArray := range schema.Dependencies {
			allErrs.AppendErrors(ValidateCustomResourceDefinitionOpenAPISchema(jsonSchemaPropsOrStringArray.Schema, fldPath.Child("dependencies").Key(dependency), ssv, false, opts, nil))
		}
	}

	if schema.XPreserveUnknownFields != nil && !*schema.XPreserveUnknownFields {
		allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("x-kubernetes-preserve-unknown-fields"), *schema.XPreserveUnknownFields, "must be true or undefined"))
	}

	if schema.XMapType != nil && schema.Type != "object" {
		if len(schema.Type) == 0 {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Required(fldPath.Child("type"), "must be object if x-kubernetes-map-type is specified"))
		} else {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("type"), schema.Type, "must be object if x-kubernetes-map-type is specified"))
		}
	}

	if schema.XMapType != nil && *schema.XMapType != "atomic" && *schema.XMapType != "granular" {
		allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.NotSupported(fldPath.Child("x-kubernetes-map-type"), *schema.XMapType, []string{"atomic", "granular"}))
	}

	if schema.XListType != nil && schema.Type != "array" {
		if len(schema.Type) == 0 {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Required(fldPath.Child("type"), "must be array if x-kubernetes-list-type is specified"))
		} else {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("type"), schema.Type, "must be array if x-kubernetes-list-type is specified"))
		}
	} else if opts.requireAtomicSetType && schema.XListType != nil && *schema.XListType == "set" && schema.Items != nil && schema.Items.Schema != nil { // by structural schema items are present
		is := schema.Items.Schema
		switch is.Type {
		case "array":
			if is.XListType != nil && *is.XListType != "atomic" { // atomic is the implicit default behaviour if unset, hence != atomic is wrong
				allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("items").Child("x-kubernetes-list-type"), is.XListType, "must be atomic as item of a list with x-kubernetes-list-type=set"))
			}
		case "object":
			if is.XMapType == nil || *is.XMapType != "atomic" { // granular is the implicit default behaviour if unset, hence nil and != atomic are wrong
				allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("items").Child("x-kubernetes-map-type"), is.XListType, "must be atomic as item of a list with x-kubernetes-list-type=set"))
			}
		}
	}

	if schema.XListType != nil && *schema.XListType != "atomic" && *schema.XListType != "set" && *schema.XListType != "map" {
		allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.NotSupported(fldPath.Child("x-kubernetes-list-type"), *schema.XListType, []string{"atomic", "set", "map"}))
	}

	if len(schema.XListMapKeys) > 0 {
		if schema.XListType == nil {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Required(fldPath.Child("x-kubernetes-list-type"), "must be map if x-kubernetes-list-map-keys is non-empty"))
		} else if *schema.XListType != "map" {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("x-kubernetes-list-type"), *schema.XListType, "must be map if x-kubernetes-list-map-keys is non-empty"))
		}
	}

	if schema.XListType != nil && *schema.XListType == "map" {
		if len(schema.XListMapKeys) == 0 {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Required(fldPath.Child("x-kubernetes-list-map-keys"), "must not be empty if x-kubernetes-list-type is map"))
		}

		if schema.Items == nil {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Required(fldPath.Child("items"), "must have a schema if x-kubernetes-list-type is map"))
		}

		if schema.Items != nil && schema.Items.Schema == nil {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("items"), schema.Items, "must only have a single schema if x-kubernetes-list-type is map"))
		}

		if schema.Items != nil && schema.Items.Schema != nil && schema.Items.Schema.Type != "object" {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("items").Child("type"), schema.Items.Schema.Type, "must be object if parent array's x-kubernetes-list-type is map"))
		}

		if schema.Items != nil && schema.Items.Schema != nil && schema.Items.Schema.Type == "object" {
			keys := map[string]struct{}{}
			for _, k := range schema.XListMapKeys {
				if s, ok := schema.Items.Schema.Properties[k]; ok {
					if s.Type == "array" || s.Type == "object" {
						allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("items").Child("properties").Key(k).Child("type"), schema.Items.Schema.Type, "must be a scalar type if parent array's x-kubernetes-list-type is map"))
					}
				} else {
					allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("x-kubernetes-list-map-keys"), schema.XListMapKeys, "entries must all be names of item properties"))
				}
				if _, ok := keys[k]; ok {
					allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("x-kubernetes-list-map-keys"), schema.XListMapKeys, "must not contain duplicate entries"))
				}
				keys[k] = struct{}{}
			}
		}
	}

	if opts.requireMapListKeysMapSetValidation {
		allErrs.SchemaErrors = append(allErrs.SchemaErrors, validateMapListKeysMapSet(schema, fldPath)...)
	}

	if len(schema.XValidations) > 0 {
		for i, rule := range schema.XValidations {
			trimmedRule := strings.TrimSpace(rule.Rule)
			trimmedMsg := strings.TrimSpace(rule.Message)
			if len(trimmedRule) == 0 {
				allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Required(fldPath.Child("x-kubernetes-validations").Index(i).Child("rule"), "rule is not specified"))
			} else if len(rule.Message) > 0 && len(trimmedMsg) == 0 {
				allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("x-kubernetes-validations").Index(i).Child("message"), rule.Message, "message must be non-empty if specified"))
			} else if hasNewlines(trimmedMsg) {
				allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("x-kubernetes-validations").Index(i).Child("message"), rule.Message, "message must not contain line breaks"))
			} else if hasNewlines(trimmedRule) && len(trimmedMsg) == 0 {
				allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Required(fldPath.Child("x-kubernetes-validations").Index(i).Child("message"), "message must be specified if rule contains line breaks"))
			}
		}

		// If any schema related validation errors have been found at this level or deeper, skip CEL expression validation.
		// Invalid OpenAPISchemas are not always possible to convert into valid CEL DeclTypes, and can lead to CEL
		// validation error messages that are not actionable (will go away once the schema errors are resolved) and that
		// are difficult for CEL expression authors to understand.
		if len(allErrs.SchemaErrors) == 0 && celContext != nil {
			typeInfo, err := celContext.TypeInfo()
			if err != nil {
				allErrs.CELErrors = append(allErrs.CELErrors, field.InternalError(fldPath.Child("x-kubernetes-validations"), fmt.Errorf("internal error: failed to construct type information for x-kubernetes-validations rules: %s", err)))
			} else if typeInfo == nil {
				allErrs.CELErrors = append(allErrs.CELErrors, field.InternalError(fldPath.Child("x-kubernetes-validations"), fmt.Errorf("internal error: failed to retrieve type information for x-kubernetes-validations")))
			} else {
				compResults, err := cel.Compile(typeInfo.Schema, typeInfo.DeclType, cel.PerCallLimit)
				if err != nil {
					allErrs.CELErrors = append(allErrs.CELErrors, field.InternalError(fldPath.Child("x-kubernetes-validations"), err))
				} else {
					for i, cr := range compResults {
						expressionCost := getExpressionCost(cr, celContext)
						if expressionCost > StaticEstimatedCostLimit {
							costErrorMsg := getCostErrorMessage("estimated rule cost", expressionCost, StaticEstimatedCostLimit)
							allErrs.CELErrors = append(allErrs.CELErrors, field.Forbidden(fldPath.Child("x-kubernetes-validations").Index(i).Child("rule"), costErrorMsg))
						}
						if celContext.TotalCost != nil {
							celContext.TotalCost.ObserveExpressionCost(fldPath.Child("x-kubernetes-validations").Index(i).Child("rule"), expressionCost)
						}
						if cr.Error != nil {
							if cr.Error.Type == apiservercel.ErrorTypeRequired {
								allErrs.CELErrors = append(allErrs.CELErrors, field.Required(fldPath.Child("x-kubernetes-validations").Index(i).Child("rule"), cr.Error.Detail))
							} else {
								allErrs.CELErrors = append(allErrs.CELErrors, field.Invalid(fldPath.Child("x-kubernetes-validations").Index(i).Child("rule"), schema.XValidations[i], cr.Error.Detail))
							}
						}
						if cr.TransitionRule {
							if uncorrelatablePath := ssv.forbidOldSelfValidations(); uncorrelatablePath != nil {
								allErrs.CELErrors = append(allErrs.CELErrors, field.Invalid(fldPath.Child("x-kubernetes-validations").Index(i).Child("rule"), schema.XValidations[i].Rule, fmt.Sprintf("oldSelf cannot be used on the uncorrelatable portion of the schema within %v", uncorrelatablePath)))
							}
						}
					}
				}
			}
		}
	}

	return allErrs
}

// multiplyWithOverflowGuard returns the product of baseCost and cardinality unless that product
// would exceed math.MaxUint, in which case math.MaxUint is returned.
func multiplyWithOverflowGuard(baseCost, cardinality uint64) uint64 {
	if baseCost == 0 {
		// an empty rule can return 0, so guard for that here
		return 0
	} else if math.MaxUint/baseCost < cardinality {
		return math.MaxUint
	}
	return baseCost * cardinality
}

func getExpressionCost(cr cel.CompilationResult, cardinalityCost *apiextensionsvalidation.CELSchemaContext) uint64 {
	if cardinalityCost.MaxCardinality != unbounded {
		return multiplyWithOverflowGuard(cr.MaxCost, *cardinalityCost.MaxCardinality)
	}
	return multiplyWithOverflowGuard(cr.MaxCost, cr.MaxCardinality)
}

func getCostErrorMessage(costName string, expressionCost, costLimit uint64) string {
	exceedFactor := float64(expressionCost) / float64(costLimit)
	var factor string
	if exceedFactor > 100.0 {
		// if exceedFactor is greater than 2 orders of magnitude, the rule is likely O(n^2) or worse
		// and will probably never validate without some set limits
		// also in such cases the cost estimation is generally large enough to not add any value
		factor = "more than 100x"
	} else if exceedFactor < 1.5 {
		factor = fmt.Sprintf("%fx", exceedFactor) // avoid reporting "exceeds budge by a factor of 1.0x"
	} else {
		factor = fmt.Sprintf("%.1fx", exceedFactor)
	}
	return fmt.Sprintf("%s exceeds budget by factor of %s (try simplifying the rule, or adding maxItems, maxProperties, and maxLength where arrays, maps, and strings are declared)", costName, factor)
}

var newlineMatcher = regexp.MustCompile(`[\n\r]+`) // valid newline chars in CEL grammar
func hasNewlines(s string) bool {
	return newlineMatcher.MatchString(s)
}

func validateMapListKeysMapSet(schema *apiextensions.JSONSchemaProps, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if schema.Items == nil || schema.Items.Schema == nil {
		return nil
	}
	if schema.XListType == nil {
		return nil
	}
	if *schema.XListType != "set" && *schema.XListType != "map" {
		return nil
	}

	// set and map list items cannot be nullable
	if schema.Items.Schema.Nullable {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("items").Child("nullable"), "cannot be nullable when x-kubernetes-list-type is "+*schema.XListType))
	}

	switch *schema.XListType {
	case "map":
		// ensure all map keys are required or have a default
		isRequired := make(map[string]bool, len(schema.Items.Schema.Required))
		for _, required := range schema.Items.Schema.Required {
			isRequired[required] = true
		}

		for _, k := range schema.XListMapKeys {
			obj, ok := schema.Items.Schema.Properties[k]
			if !ok {
				// we validate that all XListMapKeys are existing properties in ValidateCustomResourceDefinitionOpenAPISchema, so skipping here is ok
				continue
			}

			if !isRequired[k] && obj.Default == nil {
				allErrs = append(allErrs, field.Required(fldPath.Child("items").Child("properties").Key(k).Child("default"), "this property is in x-kubernetes-list-map-keys, so it must have a default or be a required property"))
			}

			if obj.Nullable {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("items").Child("properties").Key(k).Child("nullable"), "this property is in x-kubernetes-list-map-keys, so it cannot be nullable"))
			}
		}
	case "set":
		// no other set-specific validation
	}

	return allErrs
}

type specStandardValidatorV3 struct {
	allowDefaults                       bool
	disallowDefaultsReason              string
	isInsideResourceMeta                bool
	requireValidPropertyType            bool
	uncorrelatableOldSelfValidationPath *field.Path
}

func (v *specStandardValidatorV3) withForbiddenDefaults(reason string) specStandardValidator {
	clone := *v
	clone.disallowDefaultsReason = reason
	clone.allowDefaults = false
	return &clone
}

func (v *specStandardValidatorV3) withInsideResourceMeta() specStandardValidator {
	clone := *v
	clone.isInsideResourceMeta = true
	return &clone
}

func (v *specStandardValidatorV3) insideResourceMeta() bool {
	return v.isInsideResourceMeta
}

func (v *specStandardValidatorV3) withForbidOldSelfValidations(path *field.Path) specStandardValidator {
	if v.uncorrelatableOldSelfValidationPath != nil {
		// oldSelf validations are already forbidden. preserve the highest-level path
		// causing oldSelf validations to be forbidden
		return v
	}
	clone := *v
	clone.uncorrelatableOldSelfValidationPath = path
	return &clone
}

func (v *specStandardValidatorV3) forbidOldSelfValidations() *field.Path {
	return v.uncorrelatableOldSelfValidationPath
}

// validate validates against OpenAPI Schema v3.
func (v *specStandardValidatorV3) validate(schema *apiextensions.JSONSchemaProps, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if schema == nil {
		return allErrs
	}

	//
	// WARNING: if anything new is allowed below, NewStructural must be adapted to support it.
	//

	if v.requireValidPropertyType && len(schema.Type) > 0 && !openapiV3Types.Has(schema.Type) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), schema.Type, openapiV3Types.List()))
	}

	if schema.Default != nil && !v.allowDefaults {
		detail := "must not be set"
		if len(v.disallowDefaultsReason) > 0 {
			detail += " " + v.disallowDefaultsReason
		}
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("default"), detail))
	}

	if schema.ID != "" {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("id"), "id is not supported"))
	}

	if schema.AdditionalItems != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("additionalItems"), "additionalItems is not supported"))
	}

	if len(schema.PatternProperties) != 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("patternProperties"), "patternProperties is not supported"))
	}

	if len(schema.Definitions) != 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("definitions"), "definitions is not supported"))
	}

	if schema.Dependencies != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("dependencies"), "dependencies is not supported"))
	}

	if schema.Ref != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("$ref"), "$ref is not supported"))
	}

	if schema.Type == "null" {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("type"), "type cannot be set to null, use nullable as an alternative"))
	}

	if schema.Items != nil && len(schema.Items.JSONSchemas) != 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("items"), "items must be a schema object and not an array"))
	}

	if v.isInsideResourceMeta && schema.XEmbeddedResource {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("x-kubernetes-embedded-resource"), "must not be used inside of resource meta"))
	}

	return allErrs
}

// ValidateCustomResourceDefinitionSubresources statically validates
func ValidateCustomResourceDefinitionSubresources(subresources *apiextensions.CustomResourceSubresources, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if subresources == nil {
		return allErrs
	}

	if subresources.Scale != nil {
		if len(subresources.Scale.SpecReplicasPath) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("scale.specReplicasPath"), ""))
		} else {
			// should be constrained json path under .spec
			if errs := validateSimpleJSONPath(subresources.Scale.SpecReplicasPath, fldPath.Child("scale.specReplicasPath")); len(errs) > 0 {
				allErrs = append(allErrs, errs...)
			} else if !strings.HasPrefix(subresources.Scale.SpecReplicasPath, ".spec.") {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("scale.specReplicasPath"), subresources.Scale.SpecReplicasPath, "should be a json path under .spec"))
			}
		}

		if len(subresources.Scale.StatusReplicasPath) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("scale.statusReplicasPath"), ""))
		} else {
			// should be constrained json path under .status
			if errs := validateSimpleJSONPath(subresources.Scale.StatusReplicasPath, fldPath.Child("scale.statusReplicasPath")); len(errs) > 0 {
				allErrs = append(allErrs, errs...)
			} else if !strings.HasPrefix(subresources.Scale.StatusReplicasPath, ".status.") {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("scale.statusReplicasPath"), subresources.Scale.StatusReplicasPath, "should be a json path under .status"))
			}
		}

		// if labelSelectorPath is present, it should be a constrained json path under .status
		if subresources.Scale.LabelSelectorPath != nil && len(*subresources.Scale.LabelSelectorPath) > 0 {
			if errs := validateSimpleJSONPath(*subresources.Scale.LabelSelectorPath, fldPath.Child("scale.labelSelectorPath")); len(errs) > 0 {
				allErrs = append(allErrs, errs...)
			} else if !strings.HasPrefix(*subresources.Scale.LabelSelectorPath, ".spec.") && !strings.HasPrefix(*subresources.Scale.LabelSelectorPath, ".status.") {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("scale.labelSelectorPath"), subresources.Scale.LabelSelectorPath, "should be a json path under either .spec or .status"))
			}
		}
	}

	return allErrs
}

func validateSimpleJSONPath(s string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	switch {
	case len(s) == 0:
		allErrs = append(allErrs, field.Invalid(fldPath, s, "must not be empty"))
	case s[0] != '.':
		allErrs = append(allErrs, field.Invalid(fldPath, s, "must be a simple json path starting with ."))
	case s != ".":
		if cs := strings.Split(s[1:], "."); len(cs) < 1 {
			allErrs = append(allErrs, field.Invalid(fldPath, s, "must be a json path in the dot notation"))
		}
	}

	return allErrs
}

var allowedFieldsAtRootSchema = []string{"Description", "Type", "Format", "Title", "Maximum", "ExclusiveMaximum", "Minimum", "ExclusiveMinimum", "MaxLength", "MinLength", "Pattern", "MaxItems", "MinItems", "UniqueItems", "MultipleOf", "Required", "Items", "Properties", "ExternalDocs", "Example", "XPreserveUnknownFields", "XValidations"}

func allowedAtRootSchema(field string) bool {
	for _, v := range allowedFieldsAtRootSchema {
		if field == v {
			return true
		}
	}
	return false
}

func HasSchemaWith(spec *apiextensions.CustomResourceDefinitionSpec, pred func(s *apiextensions.JSONSchemaProps) bool) bool {
	if spec.Validation != nil && spec.Validation.OpenAPIV3Schema != nil && pred(spec.Validation.OpenAPIV3Schema) {
		return true
	}
	for _, v := range spec.Versions {
		if v.Schema != nil && v.Schema.OpenAPIV3Schema != nil && pred(v.Schema.OpenAPIV3Schema) {
			return true
		}
	}
	return false
}

func SchemaHas(s *apiextensions.JSONSchemaProps, pred func(s *apiextensions.JSONSchemaProps) bool) bool {
	if s == nil {
		return false
	}

	if pred(s) {
		return true
	}

	if s.Items != nil {
		if s.Items != nil && SchemaHas(s.Items.Schema, pred) {
			return true
		}
		for i := range s.Items.JSONSchemas {
			if SchemaHas(&s.Items.JSONSchemas[i], pred) {
				return true
			}
		}
	}
	for i := range s.AllOf {
		if SchemaHas(&s.AllOf[i], pred) {
			return true
		}
	}
	for i := range s.AnyOf {
		if SchemaHas(&s.AnyOf[i], pred) {
			return true
		}
	}
	for i := range s.OneOf {
		if SchemaHas(&s.OneOf[i], pred) {
			return true
		}
	}
	if SchemaHas(s.Not, pred) {
		return true
	}
	for _, s := range s.Properties {
		if SchemaHas(&s, pred) {
			return true
		}
	}
	if s.AdditionalProperties != nil {
		if SchemaHas(s.AdditionalProperties.Schema, pred) {
			return true
		}
	}
	for _, s := range s.PatternProperties {
		if SchemaHas(&s, pred) {
			return true
		}
	}
	if s.AdditionalItems != nil {
		if SchemaHas(s.AdditionalItems.Schema, pred) {
			return true
		}
	}
	for _, s := range s.Definitions {
		if SchemaHas(&s, pred) {
			return true
		}
	}
	for _, d := range s.Dependencies {
		if SchemaHas(d.Schema, pred) {
			return true
		}
	}

	return false
}
