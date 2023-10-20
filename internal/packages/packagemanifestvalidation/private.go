package packagemanifestvalidation

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsvalidation "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	structuraldefaulting "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	apiservervalidation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiserverapiscel "k8s.io/apiserver/pkg/apis/cel"
	apiservercel "k8s.io/apiserver/pkg/cel"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	kopenapivalidation "k8s.io/kube-openapi/pkg/validation/validate"

	"package-operator.run/internal/apis/manifests"
)

var metaFields = sets.NewString("metadata", "kind", "apiVersion")

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

// validationOptions groups several validation options, to avoid passing multiple bool parameters to methods.
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

const (
	// staticEstimatedCostLimit represents the largest-allowed static CEL cost on a per-expression basis.
	staticEstimatedCostLimit = 10000000
	// staticEstimatedCRDCostLimit represents the largest-allowed total cost for the x-kubernetes-validations rules of a CRD.
	staticEstimatedCRDCostLimit = 100000000
)

// validateCustomResourceDefinitionValidation statically validates
// context is passed for supporting context cancellation during cel validation when validating defaults.
func validateCustomResourceDefinitionValidation(ctx context.Context, customResourceValidation *apiextensions.CustomResourceValidation, statusSubresourceEnabled bool, opts validationOptions, fldPath *field.Path) (allErrs field.ErrorList) {
	allErrs = field.ErrorList{}

	if customResourceValidation == nil {
		return allErrs
	}
	schema := customResourceValidation.OpenAPIV3Schema
	if schema == nil {
		if _, _, err := apiservervalidation.NewSchemaValidator(customResourceValidation.OpenAPIV3Schema); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, "", fmt.Sprintf("error building validator: %v", err)))
		}
		return allErrs
	}

	// if the status subresource is enabled, only certain fields are allowed inside the root schema.
	// these fields are chosen such that, if status is extracted as properties["status"], it's validation is not lost.
	if statusSubresourceEnabled {
		if err := statusSubresource(schema, fldPath); err != nil {
			allErrs = append(allErrs, err)
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
		var extraAllErrs field.ErrorList
		celContext, structuralSchemaInitErrs, extraAllErrs = requirestructuralSchema(ctx, schema, opts, fldPath)
		allErrs = append(allErrs, extraAllErrs...)
	}
	allErrs = append(allErrs, validateCustomResourceDefinitionOpenAPISchema(schema, fldPath.Child("openAPIV3Schema"), openAPIV3Schema, true, &opts, celContext).allErrors()...)

	if len(allErrs) == 0 && len(structuralSchemaInitErrs) > 0 {
		// Structural schema initialization errors overlap with OpenAPISchema validation errors so we only show them
		// if there are no OpenAPISchema validation errors.
		allErrs = append(allErrs, structuralSchemaInitErrs...)
	}

	if celContext != nil && celContext.TotalCost != nil {
		if celContext.TotalCost.Total > staticEstimatedCRDCostLimit {
			for _, expensive := range celContext.TotalCost.MostExpensive {
				costErrorMsg := "contributed to estimated rule cost total exceeding cost limit for entire OpenAPIv3 schema"
				allErrs = append(allErrs, field.Forbidden(expensive.Path, costErrorMsg))
			}

			costErrorMsg := getCostErrorMessage("x-kubernetes-validations estimated rule cost total for entire OpenAPIv3 schema", celContext.TotalCost.Total, staticEstimatedCRDCostLimit)
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("openAPIV3Schema"), costErrorMsg))
		}
	}

	// if validation passed otherwise, make sure we can actually construct a schema validator from this custom resource validation.
	if len(allErrs) == 0 {
		if _, _, err := apiservervalidation.NewSchemaValidator(customResourceValidation.OpenAPIV3Schema); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, "", fmt.Sprintf("error building validator: %v", err)))
		}
	}
	return allErrs
}

func statusSubresource(schema *apiextensions.JSONSchemaProps, fldPath *field.Path) *field.Error {
	v := reflect.ValueOf(schema).Elem()
	for i := 0; i < v.NumField(); i++ {
		// skip zero values
		if value := v.Field(i).Interface(); reflect.DeepEqual(value, reflect.Zero(reflect.TypeOf(value)).Interface()) {
			continue
		}

		fieldName := v.Type().Field(i).Name

		// only "object" type is valid at root of the schema since validation schema for status is extracted as properties["status"]
		if fieldName == "Type" {
			if schema.Type != OpenapiV3TypeObject {
				return field.Invalid(fldPath.Child("openAPIV3Schema.type"), schema.Type, `only "object" is allowed as the type at the root of the schema if the status subresource is enabled`)
			}
			continue
		}

		if !allowedAtRootSchema(fieldName) {
			return field.Invalid(fldPath.Child("openAPIV3Schema"), *schema, fmt.Sprintf(`only %v fields are allowed at the root of the schema if the status subresource is enabled`, allowedFieldsAtRootSchema))
		}
	}

	return nil
}

func requirestructuralSchema(ctx context.Context, schema *apiextensions.JSONSchemaProps, opts validationOptions, fldPath *field.Path) (*apiextensionsvalidation.CELSchemaContext, field.ErrorList, field.ErrorList) {
	ss, err := structuralschema.NewStructural(schema)
	if err != nil {
		// These validation errors overlap with  OpenAPISchema validation errors so we keep track of them
		// separately and only show them if OpenAPISchema validation does not report any errors.
		return nil, field.ErrorList{field.Invalid(fldPath.Child("openAPIV3Schema"), "", err.Error())}, nil
	}

	validationErrors := structuralschema.ValidateStructural(fldPath.Child("openAPIV3Schema"), ss)
	if len(validationErrors) > 0 {
		return nil, nil, validationErrors
	}

	validationErrors, err = structuraldefaulting.ValidateDefaults(ctx, fldPath.Child("openAPIV3Schema"), ss, true, opts.requirePrunedDefaults)
	if err != nil {
		// this should never happen
		return nil, nil, field.ErrorList{field.Invalid(fldPath.Child("openAPIV3Schema"), "", err.Error())}
	}
	if len(validationErrors) > 0 {
		return nil, nil, validationErrors
	}

	// Only initialize CEL rule validation context if the structural schemas are valid.
	// A nil CELSchemaContext indicates that no CEL validation should be attempted.
	return apiextensionsvalidation.RootCELContext(schema), nil, nil
}

// validateCustomResourceDefinitionOpenAPISchema statically validates.
func validateCustomResourceDefinitionOpenAPISchema(schema *apiextensions.JSONSchemaProps, fldPath *field.Path, ssv specStandardValidator, isRoot bool, opts *validationOptions, celContext *apiextensionsvalidation.CELSchemaContext) *OpenAPISchemaErrorList {
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
		allErrs.appendErrors(validateCustomResourceDefinitionOpenAPISchema(schema.AdditionalProperties.Schema, fldPath.Child("additionalProperties"), subSsv, false, opts, celContext.ChildAdditionalPropertiesContext(schema.AdditionalProperties.Schema)))
	}

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
		allErrs.appendErrors(validateCustomResourceDefinitionOpenAPISchema(&propertySchema, fldPath.Child("properties").Key(property), subSsv, false, opts, celContext.ChildPropertyContext(&propertySchema, property)))
	}

	allErrs.appendErrors(validateCustomResourceDefinitionOpenAPISchema(schema.Not, fldPath.Child("not"), ssv, false, opts, nil))

	for i, jsonSchema := range schema.AllOf {
		allOfSchema := jsonSchema
		allErrs.appendErrors(validateCustomResourceDefinitionOpenAPISchema(&allOfSchema, fldPath.Child("allOf").Index(i), ssv, false, opts, nil))
	}

	for i, jsonSchema := range schema.OneOf {
		oneOfSchema := jsonSchema
		allErrs.appendErrors(validateCustomResourceDefinitionOpenAPISchema(&oneOfSchema, fldPath.Child("oneOf").Index(i), ssv, false, opts, nil))
	}

	for i, jsonSchema := range schema.AnyOf {
		anyOfSchema := jsonSchema
		allErrs.appendErrors(validateCustomResourceDefinitionOpenAPISchema(&anyOfSchema, fldPath.Child("anyOf").Index(i), ssv, false, opts, nil))
	}

	for definition, jsonSchema := range schema.Definitions {
		definitionSchema := jsonSchema
		allErrs.appendErrors(validateCustomResourceDefinitionOpenAPISchema(&definitionSchema, fldPath.Child("definitions").Key(definition), ssv, false, opts, nil))
	}

	if schema.Items != nil {
		subSsv := ssv

		// we can only correlate old/new items for "map" and "set" lists, and correlation of
		// "set" elements by identity is not supported for cel (x-kubernetes-validations)
		// rules. an unset list type defaults to "atomic".
		if schema.XListType == nil || *schema.XListType != xListTypeMap {
			subSsv = subSsv.withForbidOldSelfValidations(fldPath)
		}

		allErrs.appendErrors(validateCustomResourceDefinitionOpenAPISchema(schema.Items.Schema, fldPath.Child("items"), subSsv, false, opts, celContext.ChildItemsContext(schema.Items.Schema)))
		if len(schema.Items.JSONSchemas) != 0 {
			for i, jsonSchema := range schema.Items.JSONSchemas {
				itemsSchema := jsonSchema
				allErrs.appendErrors(validateCustomResourceDefinitionOpenAPISchema(&itemsSchema, fldPath.Child("items").Index(i), subSsv, false, opts, celContext.ChildItemsContext(&itemsSchema)))
			}
		}
	}

	for dependency, jsonSchemaPropsOrStringArray := range schema.Dependencies {
		allErrs.appendErrors(validateCustomResourceDefinitionOpenAPISchema(jsonSchemaPropsOrStringArray.Schema, fldPath.Child("dependencies").Key(dependency), ssv, false, opts, nil))
	}

	if schema.XPreserveUnknownFields != nil && !*schema.XPreserveUnknownFields {
		allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("x-kubernetes-preserve-unknown-fields"), *schema.XPreserveUnknownFields, "must be true or undefined"))
	}

	allErrs.appendErrors(xlisttypenotnil(schema, opts, fldPath))
	allErrs.appendErrors(xmaptypenotnil(schema, fldPath))

	if len(schema.XListMapKeys) > 0 {
		if schema.XListType == nil {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Required(fldPath.Child("x-kubernetes-list-type"), "must be map if x-kubernetes-list-map-keys is non-empty"))
		} else if *schema.XListType != xListTypeMap {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("x-kubernetes-list-type"), *schema.XListType, "must be map if x-kubernetes-list-map-keys is non-empty"))
		}
	}

	if opts.requireMapListKeysMapSetValidation {
		allErrs.SchemaErrors = append(allErrs.SchemaErrors, validateMapListKeysMapSet(schema, fldPath)...)
	}

	allErrs.appendErrors(validateSchemaStuffWithXPrefixedName(schema, fldPath, ssv, celContext))

	return allErrs
}

func xmaptypenotnil(schema *apiextensions.JSONSchemaProps, fldPath *field.Path) *OpenAPISchemaErrorList {
	allErrs := &OpenAPISchemaErrorList{field.ErrorList{}, field.ErrorList{}}
	if schema.XMapType == nil {
		return allErrs
	}

	if schema.Type != OpenapiV3TypeObject {
		if len(schema.Type) == 0 {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Required(fldPath.Child("type"), "must be object if x-kubernetes-map-type is specified"))
		} else {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("type"), schema.Type, "must be object if x-kubernetes-map-type is specified"))
		}
	}

	if *schema.XMapType != "atomic" && *schema.XMapType != "granular" {
		allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.NotSupported(fldPath.Child("x-kubernetes-map-type"), *schema.XMapType, []string{"atomic", "granular"}))
	}

	return allErrs
}

func xlisttypenotnil(schema *apiextensions.JSONSchemaProps, opts *validationOptions, fldPath *field.Path) *OpenAPISchemaErrorList {
	allErrs := &OpenAPISchemaErrorList{field.ErrorList{}, field.ErrorList{}}
	if schema.XListType == nil {
		return allErrs
	}
	if schema.Type != xListTypeArray {
		if len(schema.Type) == 0 {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Required(fldPath.Child("type"), "must be array if x-kubernetes-list-type is specified"))
		} else {
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("type"), schema.Type, "must be array if x-kubernetes-list-type is specified"))
		}
	}

	if schema.Type == xListTypeArray && opts.requireAtomicSetType && *schema.XListType == xListTypeSet && schema.Items != nil && schema.Items.Schema != nil { // by structural schema items are present
		is := schema.Items.Schema
		switch is.Type {
		case openapiV3TypeArray:
			if is.XListType != nil && *is.XListType != xListTypeAtomic { // atomic is the implicit default behaviour if unset, hence != atomic is wrong
				allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("items").Child("x-kubernetes-list-type"), is.XListType, "must be atomic as item of a list with x-kubernetes-list-type=set"))
			}
		case OpenapiV3TypeObject:
			if is.XMapType == nil || *is.XMapType != "atomic" { // granular is the implicit default behaviour if unset, hence nil and != atomic are wrong
				allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("items").Child("x-kubernetes-map-type"), is.XListType, "must be atomic as item of a list with x-kubernetes-list-type=set"))
			}
		}
	}

	if *schema.XListType != xListTypeAtomic && *schema.XListType != xListTypeSet && *schema.XListType != xListTypeMap {
		allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.NotSupported(fldPath.Child("x-kubernetes-list-type"), *schema.XListType, []string{xListTypeAtomic, xListTypeSet, xListTypeMap}))
	}

	if *schema.XListType == xListTypeMap {
		extraErrs := validateXListTypeMap(schema, fldPath)
		allErrs.CELErrors = append(allErrs.CELErrors, extraErrs.CELErrors...)
		allErrs.SchemaErrors = append(allErrs.SchemaErrors, extraErrs.SchemaErrors...)
	}

	return allErrs
}

func schemaitemsnotNil(schema *apiextensions.JSONSchemaProps, fldPath *field.Path) *OpenAPISchemaErrorList {
	allErrs := &OpenAPISchemaErrorList{field.ErrorList{}, field.ErrorList{}}
	if schema.Items.Schema == nil {
		allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("items"), schema.Items, "must only have a single schema if x-kubernetes-list-type is map"))
	}

	if schema.Items.Schema != nil && schema.Items.Schema.Type != OpenapiV3TypeObject {
		allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("items").Child("type"), schema.Items.Schema.Type, "must be object if parent array's x-kubernetes-list-type is map"))
	}

	return allErrs
}

func validateXListTypeMap(schema *apiextensions.JSONSchemaProps, fldPath *field.Path) *OpenAPISchemaErrorList {
	allErrs := &OpenAPISchemaErrorList{field.ErrorList{}, field.ErrorList{}}
	if len(schema.XListMapKeys) == 0 {
		allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Required(fldPath.Child("x-kubernetes-list-map-keys"), "must not be empty if x-kubernetes-list-type is map"))
	}

	if schema.Items == nil {
		allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Required(fldPath.Child("items"), "must have a schema if x-kubernetes-list-type is map"))
	} else {
		extra := schemaitemsnotNil(schema, fldPath)
		allErrs.CELErrors = append(allErrs.CELErrors, extra.CELErrors...)
		allErrs.SchemaErrors = append(allErrs.SchemaErrors, extra.SchemaErrors...)
	}

	if schema.Items == nil || schema.Items.Schema == nil || schema.Items.Schema.Type != OpenapiV3TypeObject {
		return allErrs
	}

	keys := map[string]struct{}{}
	for _, k := range schema.XListMapKeys {
		if s, ok := schema.Items.Schema.Properties[k]; ok {
			if s.Type == openapiV3TypeArray || s.Type == OpenapiV3TypeObject {
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

	return allErrs
}

func validateSchemaStuffWithXPrefixedName(schema *apiextensions.JSONSchemaProps, fldPath *field.Path, ssv specStandardValidator, celContext *apiextensionsvalidation.CELSchemaContext) *OpenAPISchemaErrorList {
	allErrs := &OpenAPISchemaErrorList{SchemaErrors: field.ErrorList{}, CELErrors: field.ErrorList{}}

	if len(schema.XValidations) == 0 {
		return allErrs
	}

	for i, rule := range schema.XValidations {
		trimmedRule := strings.TrimSpace(rule.Rule)
		trimmedMsg := strings.TrimSpace(rule.Message)
		switch {
		case len(trimmedRule) == 0:
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Required(fldPath.Child("x-kubernetes-validations").Index(i).Child("rule"), "rule is not specified"))
		case len(rule.Message) > 0 && len(trimmedMsg) == 0:
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("x-kubernetes-validations").Index(i).Child("message"), rule.Message, "message must be non-empty if specified"))
		case hasNewlines(trimmedMsg):
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Invalid(fldPath.Child("x-kubernetes-validations").Index(i).Child("message"), rule.Message, "message must not contain line breaks"))
		case hasNewlines(trimmedRule) && len(trimmedMsg) == 0:
			allErrs.SchemaErrors = append(allErrs.SchemaErrors, field.Required(fldPath.Child("x-kubernetes-validations").Index(i).Child("message"), "message must be specified if rule contains line breaks"))
		}
	}

	// If any schema related validation errors have been found at this level or deeper, skip CEL expression validation.
	// Invalid OpenAPISchemas are not always possible to convert into valid CEL DeclTypes, and can lead to CEL
	// validation error messages that are not actionable (will go away once the schema errors are resolved) and that
	// are difficult for CEL expression authors to understand.
	if len(allErrs.SchemaErrors) != 0 || celContext == nil {
		return allErrs
	}
	typeInfo, err := celContext.TypeInfo()
	switch {
	case err != nil:
		allErrs.CELErrors = append(allErrs.CELErrors, field.InternalError(fldPath.Child("x-kubernetes-validations"), fmt.Errorf("internal error: failed to construct type information for x-kubernetes-validations rules: %w", err)))
	case typeInfo == nil:
		allErrs.CELErrors = append(allErrs.CELErrors, field.InternalError(fldPath.Child("x-kubernetes-validations"), fmt.Errorf("internal error: %w", ErrXKubernetesValidations)))
	default:
		compResults, err := cel.Compile(typeInfo.Schema, typeInfo.DeclType, apiserverapiscel.PerCallLimit, nil, nil)
		if err != nil {
			allErrs.CELErrors = append(allErrs.CELErrors, field.InternalError(fldPath.Child("x-kubernetes-validations"), err))
			return allErrs
		}
		for i, cr := range compResults {
			expressionCost := getExpressionCost(cr, celContext)
			if expressionCost > staticEstimatedCostLimit {
				costErrorMsg := getCostErrorMessage("estimated rule cost", expressionCost, staticEstimatedCostLimit)
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

	return allErrs
}

func validatePackageManifestConfig(ctx context.Context, config *manifests.PackageManifestSpecConfig, fldPath *field.Path) field.ErrorList {
	if config.OpenAPIV3Schema == nil {
		return nil
	}

	var allErrs field.ErrorList
	schema := config.OpenAPIV3Schema
	if schema.Nullable {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("openAPIV3Schema.nullable"), "nullable cannot be true at the root"))
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
			OpenAPIV3Schema: schema,
		}, false, opts, fldPath)...)
	return allErrs
}

func validatePackageConfigurationBySchema(_ context.Context, schema *apiextensions.JSONSchemaProps, config map[string]interface{}, fldPath *field.Path) (field.ErrorList, error) {
	if schema == nil {
		return nil, nil
	}

	openapiSchema := &spec.Schema{}
	if err := apiservervalidation.ConvertJSONSchemaProps(schema, openapiSchema); err != nil {
		return nil, err
	}

	v := kopenapivalidation.NewSchemaValidator(openapiSchema, nil, "", strfmt.Default)
	return apiservervalidation.ValidateCustomResource(fldPath, config, v), nil
}
