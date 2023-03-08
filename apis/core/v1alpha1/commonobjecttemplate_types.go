package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ObjectTemplateSpec specification.
type ObjectTemplateSpec struct {
	// Go template of a Kubernetes manifest
	Template string `json:"template"`

	// Objects in which configuration parameters are fetched
	Sources []ObjectTemplateSource `json:"sources"`
}

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

type ObjectTemplateSourceItem struct {
	Key         string `json:"key"`
	Destination string `json:"destination"`
}

// ObjectTemplateStatus defines the observed state of a ObjectTemplate ie the status of the templated object.
type ObjectTemplateStatus struct {
	// Conditions is a list of status conditions the templated object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
