package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ObjectTemplates contain a go template of a Kubernetes manifest. This manifest is then templated with the
// sources provided in the .Spec.Sources. The sources can only come from objects within the same nampespace
// as the ObjectTemplate.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type ObjectTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObjectTemplateSpec   `json:"spec,omitempty"`
	Status ObjectTemplateStatus `json:"status,omitempty"`
}

// ObjectTemplateList contains a list of ObjectTemplates.
// +kubebuilder:object:root=true
type ObjectTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectTemplate `json:"items"`
}

func init() { register(&ObjectTemplate{}, &ObjectTemplateList{}) }
