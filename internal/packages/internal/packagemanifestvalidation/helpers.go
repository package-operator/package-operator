package packagemanifestvalidation

import (
	"fmt"
	"math"
	"regexp"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsvalidation "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	xListTypeMap        = "map"
	xListTypeSet        = "set"
	xListTypeAtomic     = "atomic"
	xListTypeArray      = "array"
	OpenapiV3TypeObject = "object"
	openapiV3TypeArray  = "array"
)

var (
	openapiV3Types = sets.NewString("string", "number", "integer", "boolean", openapiV3TypeArray, OpenapiV3TypeObject)
	// unbounded uses nil to represent an unbounded cardinality value.
	unbounded                 *uint64
	allowedFieldsAtRootSchema = []string{
		"Description", "Type", "Format", "Title", "Maximum", "ExclusiveMaximum", "Minimum",
		"ExclusiveMinimum", "MaxLength", "MinLength", "Pattern", "MaxItems", "MinItems", "UniqueItems",
		"MultipleOf", "Required", "Items", "Properties", "ExternalDocs", "Example", "XPreserveUnknownFields", "XValidations",
	}

	newlineMatcher = regexp.MustCompile(`[\n\r]+`) // valid newline chars in CEL grammar
)

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
	switch {
	case exceedFactor > 100.0:
		// if exceedFactor is greater than 2 orders of magnitude, the rule is likely O(n^2) or worse
		// and will probably never validate without some set limits
		// also in such cases the cost estimation is generally large enough to not add any value
		factor = "more than 100x"
	case exceedFactor < 1.5:
		factor = fmt.Sprintf("%fx", exceedFactor) // avoid reporting "exceeds budge by a factor of 1.0x"
	default:
		factor = fmt.Sprintf("%.1fx", exceedFactor)
	}

	return fmt.Sprintf(
		"%s exceeds budget by factor of %s (try simplifying the rule, or adding maxItems, "+
			"maxProperties, and maxLength where arrays, maps, and strings are declared)",
		costName, factor,
	)
}

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
	if *schema.XListType != xListTypeSet && *schema.XListType != xListTypeMap {
		return nil
	}

	// set and map list items cannot be nullable
	if schema.Items.Schema.Nullable {
		allErrs = append(allErrs,
			field.Forbidden(fldPath.Child("items").Child("nullable"),
				"cannot be nullable when x-kubernetes-list-type is "+*schema.XListType),
		)
	}

	switch *schema.XListType {
	case xListTypeMap:
		// ensure all map keys are required or have a default
		isRequired := make(map[string]bool, len(schema.Items.Schema.Required))
		for _, required := range schema.Items.Schema.Required {
			isRequired[required] = true
		}

		for _, k := range schema.XListMapKeys {
			obj, ok := schema.Items.Schema.Properties[k]
			if !ok {
				// we validate that all XListMapKeys are existing properties in
				// ValidateCustomResourceDefinitionOpenAPISchema, so skipping here is ok.
				continue
			}

			if !isRequired[k] && obj.Default == nil {
				allErrs = append(allErrs,
					field.Required(
						fldPath.Child("items").Child("properties").Key(k).Child("default"),
						"this property is in x-kubernetes-list-map-keys, so it must have a default or be a required property",
					),
				)
			}

			if obj.Nullable {
				allErrs = append(allErrs,
					field.Forbidden(
						fldPath.Child("items").Child("properties").Key(k).Child("nullable"),
						"this property is in x-kubernetes-list-map-keys, so it cannot be nullable",
					),
				)
			}
		}
	case xListTypeSet:
		// no other set-specific validation
	}

	return allErrs
}

func allowedAtRootSchema(field string) bool {
	for _, v := range allowedFieldsAtRootSchema {
		if field == v {
			return true
		}
	}
	return false
}
