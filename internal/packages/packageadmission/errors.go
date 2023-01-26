package packageadmission

import (
	"errors"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// OpenAPISchemaErrorList tracks all validation errors reported ValidateCustomResourceDefinitionOpenAPISchema
// with CEL related errors kept separate from schema related errors.
type OpenAPISchemaErrorList struct {
	SchemaErrors field.ErrorList
	CELErrors    field.ErrorList
}

var (
	ErrDuplicateConfig        = errors.New("config raw and object fields are both set")
	ErrXKubernetesValidations = errors.New("failed to retrieve type information for x-kubernetes-validations")
)

// appendErrors appends all errors in the provided list with the errors of this list.
func (o *OpenAPISchemaErrorList) appendErrors(list *OpenAPISchemaErrorList) {
	if o == nil || list == nil {
		return
	}
	o.SchemaErrors = append(o.SchemaErrors, list.SchemaErrors...)
	o.CELErrors = append(o.CELErrors, list.CELErrors...)
}

// allErrors returns a list containing both schema and CEL errors.
func (o *OpenAPISchemaErrorList) allErrors() field.ErrorList {
	if o == nil {
		return field.ErrorList{}
	}
	return append(o.SchemaErrors, o.CELErrors...)
}
