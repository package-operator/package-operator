package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ObjectTemplateSpec specification.
type ObjectTemplateSpec struct {
	// Go template of a Kubernetes manifest
	Template string `json:"template"`

	// Objects in which configuration parameters are fetched
	Sources []ObjectTemplateSource `json:"sources"`
}

// ObjectTemplateSource defines a source for a template.
type ObjectTemplateSource struct {
	APIVersion string                     `json:"apiVersion"`
	Kind       string                     `json:"kind"`
	Namespace  string                     `json:"namespace,omitempty"`
	Name       string                     `json:"name"`
	Items      []ObjectTemplateSourceItem `json:"items"`
	// Marks this source as optional.
	// The templated object will still be applied if optional sources are not found.
	// If the source object is created later on, it will be eventually picked up.
	Optional bool `json:"optional,omitempty"`
}

// ObjectTemplateSourceItem defines a source item for an object template.
type ObjectTemplateSourceItem struct {
	// JSONPath to value in source object.
	Key string `json:"key"`
	// JSONPath to destination in which to store copy of the source value.
	Destination string `json:"destination"`
}

// ObjectTemplateStatus defines the observed state of a ObjectTemplate ie the status of the templated object.
type ObjectTemplateStatus struct {
	// Conditions is a list of status conditions the templated object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ControllerOf references the templated object.
	ControllerOf ControlledObjectReference `json:"controllerOf,omitempty"`
	// This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// When evaluating object state in code, use .Conditions instead.
	Phase ObjectTemplateStatusPhase `json:"phase,omitempty"`
}

// ObjectTemplate condition types.
const (
	// Invalid indicates an issue with the ObjectTemplates own configuration.
	ObjectTemplateInvalid = "package-operator.run/Invalid"
)

// ObjectTemplateStatusPhase defines the status phase of an object template.
type ObjectTemplateStatusPhase string

// Well-known ObjectTemplates Phases for printing a Status in kubectl,
// see deprecation notice in ObjectTemplatesStatus for details.
const (
	ObjectTemplatePhasePending ObjectTemplateStatusPhase = "Pending"
	ObjectTemplatePhaseActive  ObjectTemplateStatusPhase = "Active"
	ObjectTemplatePhaseError   ObjectTemplateStatusPhase = "Error"
)
