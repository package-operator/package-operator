package v1alpha1

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
}

type ObjectTemplateSourceItem struct {
	Key         string `json:"key"`
	Destination string `json:"destination"`
}
